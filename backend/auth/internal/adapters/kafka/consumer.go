package kafka

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	"github.com/identicalaffiliation/web-go-project/auth/internal/ports"
)

// topicCheckTimeout ограничивает один metadata-запрос при ожидании топика.
const topicCheckTimeout = 5 * time.Second

// ErrTopicNotFound — топика ещё нет на брокере.
var ErrTopicNotFound = errors.New("topic not found")

type messageReader interface {
	FetchMessage(ctx context.Context) (kafka.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

type messageHandler interface {
	Handle(ctx context.Context, value []byte) error
}

// topicChecker проверяет, что топик существует. Он обязан не создавать топик:
// иначе ожидание теряет смысл, а опечатка в имени тихо плодила бы мусорные топики.
type topicChecker interface {
	// Exists возвращает nil, если топик есть, ErrTopicNotFound, если его нет,
	// и любую другую ошибку, если брокер недоступен.
	Exists(ctx context.Context) error
}

type ConsumerConfig struct {
	Brokers       []string
	Topic         string
	GroupID       string
	MaxWait       time.Duration
	RetryBackoff  time.Duration
	CommitTimeout time.Duration
}

type Consumer struct {
	newReader     func() messageReader
	topic         topicChecker
	handler       messageHandler
	log           ports.Logger
	retryBackoff  time.Duration
	commitTimeout time.Duration

	// reader создаётся лениво в Run, а Close вызывают из другой горутины.
	mu     sync.Mutex
	reader messageReader
	closed bool
}

func NewConsumer(cfg ConsumerConfig, handler messageHandler, log ports.Logger) *Consumer {
	log = log.With("topic", cfg.Topic, "group", cfg.GroupID)
	// Один на весь consumer: счётчик должен переживать пересоздание поколений группы.
	warnLog := newWarnThrottle(log, kafkaWarnInterval)

	newReader := func() messageReader {
		return kafka.NewReader(kafka.ReaderConfig{
			Brokers: cfg.Brokers,
			Topic:   cfg.Topic,
			GroupID: cfg.GroupID,
			MaxWait: cfg.MaxWait,
			// Новая группа должна прочитать и события, опубликованные до её первого
			// старта: иначе роль для них не выдастся никогда.
			StartOffset: kafka.FirstOffset,
			// Ошибки группы kafka-go по умолчанию глотает, и сломанный consumer
			// выглядит как здоровый. Warn, а не Error: библиотека сама повторяет
			// попытки; частоту ограничивает warnThrottle.
			ErrorLogger: kafka.LoggerFunc(warnLog.Printf),
		})
	}

	return newConsumer(newReader, brokerTopic{
		client: &kafka.Client{Addr: kafka.TCP(cfg.Brokers...), Timeout: topicCheckTimeout},
		name:   cfg.Topic,
	}, handler, log, cfg.RetryBackoff, cfg.CommitTimeout)
}

func newConsumer(
	newReader func() messageReader,
	topic topicChecker,
	handler messageHandler,
	log ports.Logger,
	retryBackoff, commitTimeout time.Duration,
) *Consumer {
	return &Consumer{
		newReader:     newReader,
		topic:         topic,
		handler:       handler,
		log:           log,
		retryBackoff:  retryBackoff,
		commitTimeout: commitTimeout,
	}
}

// Run блокируется до отмены ctx и в этом случае возвращает nil. Запускается в
// отдельной горутине под errgroup; Close вызывают после её завершения.
func (c *Consumer) Run(ctx context.Context) error {
	if !c.awaitTopic(ctx) {
		return nil
	}

	reader := c.startReader()
	if reader == nil {
		return nil
	}
	c.log.Info("role consumer started")

	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			c.log.WithError(err).Error("fetch message from kafka")
			if !c.wait(ctx) {
				return nil
			}
			continue
		}

		if !c.process(ctx, reader, msg) {
			return nil
		}
	}
}

type topicWait int

const (
	waitNone topicWait = iota
	waitMissing
	waitUnavailable
)

// awaitTopic ждёт появления топика и возвращает false, если ctx отменили раньше.
// Reader нельзя создавать до этого: группа, вошедшая в несуществующий топик,
// получает пустое назначение партиций и не просыпается, когда топик появится
// (а опция WatchPartitionChanges вместо этого крутит ребалансы без пауз).
// Лог пишется только при смене причины ожидания, а не на каждой попытке: так
// сообщений не больше двух-трёх за всё ожидание, а не по одному в retry_backoff.
func (c *Consumer) awaitTopic(ctx context.Context) bool {
	last := waitNone
	for {
		checkCtx, cancel := context.WithTimeout(ctx, topicCheckTimeout)
		err := c.topic.Exists(checkCtx)
		cancel()

		if err == nil {
			if last != waitNone {
				c.log.Info("topic is available")
			}
			return true
		}
		if ctx.Err() != nil {
			return false
		}

		state := waitUnavailable
		if errors.Is(err, ErrTopicNotFound) {
			state = waitMissing
		}
		if state != last {
			last = state
			if state == waitMissing {
				c.log.Warn("topic does not exist yet, waiting for it to be created")
			} else {
				c.log.WithError(err).Warn("kafka is unavailable, waiting for it")
			}
		}

		if !c.wait(ctx) {
			return false
		}
	}
}

// startReader создаёт reader. Возвращает nil, если Close уже вызван: иначе
// reader, созданный после Close, никто бы не закрыл.
func (c *Consumer) startReader() messageReader {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.reader = c.newReader()
	return c.reader
}

// process возвращает false, если ctx отменили раньше, чем сообщение обработано.
// Оффсет коммитится только в трёх случаях: успех, битое сообщение (повтор его
// не починит) и неизвестный пользователь. При сбое БД то же сообщение
// повторяется, а следующее не читается, поэтому оффсет не уезжает за него.
func (c *Consumer) process(ctx context.Context, reader messageReader, msg kafka.Message) bool {
	for {
		err := c.handler.Handle(ctx, msg.Value)
		switch {
		case err == nil:
		case errors.Is(err, domain.ErrInvalidEvent):
			// Тело сообщения не логируем: оно битое и может содержать что угодно.
			c.log.WithError(err).Error("skip invalid event",
				"partition", msg.Partition, "offset", msg.Offset)
		case errors.Is(err, ports.ErrUserNotFound):
			c.log.WithError(err).Warn("skip event for unknown user",
				"partition", msg.Partition, "offset", msg.Offset)
		default:
			// Ошибка из-за остановки сервиса — не сбой: сообщение останется
			// некоммитнутым и придёт после рестарта.
			if ctx.Err() != nil {
				return false
			}
			c.log.WithError(err).Error("handle event failed, will retry",
				"partition", msg.Partition, "offset", msg.Offset)
			if !c.wait(ctx) {
				return false
			}
			continue
		}

		c.commit(ctx, reader, msg)
		return true
	}
}

// commit отвязан от отмены ctx: если сигнал остановки пришёл сразу после
// успешного UPDATE, оффсет всё равно должен сохраниться. Если коммит не
// удался, сообщение придёт повторно, а обработка идемпотентна.
func (c *Consumer) commit(ctx context.Context, reader messageReader, msg kafka.Message) {
	commitCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.commitTimeout)
	defer cancel()

	if err := reader.CommitMessages(commitCtx, msg); err != nil {
		c.log.WithError(err).Error("commit kafka offset",
			"partition", msg.Partition, "offset", msg.Offset)
	}
}

// wait ждёт retryBackoff и возвращает false, если за это время отменили ctx.
func (c *Consumer) wait(ctx context.Context) bool {
	timer := time.NewTimer(c.retryBackoff)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// Close безопасен, если Run не дошёл до создания reader (топика так и не
// появилось или Close вызван до Run), и после него Run reader уже не создаст.
func (c *Consumer) Close() error {
	c.mu.Lock()
	c.closed = true
	reader := c.reader
	c.mu.Unlock()

	if reader == nil {
		return nil
	}
	if err := reader.Close(); err != nil {
		return fmt.Errorf("close kafka reader: %w", err)
	}
	return nil
}

// brokerTopic проверяет топик metadata-запросом. kafka.Client.Metadata шлёт
// запрос без AllowAutoTopicCreation, в отличие от conn.ReadPartitions(topic),
// который при включённом авто-создании брокера топик создаёт.
type brokerTopic struct {
	client *kafka.Client
	name   string
}

func (b brokerTopic) Exists(ctx context.Context) error {
	resp, err := b.client.Metadata(ctx, &kafka.MetadataRequest{Topics: []string{b.name}})
	if err != nil {
		return fmt.Errorf("fetch topic metadata: %w", err)
	}

	for _, t := range resp.Topics {
		if t.Name != b.name {
			continue
		}
		switch {
		case t.Error == nil:
			return nil
		case errors.Is(t.Error, kafka.UnknownTopicOrPartition):
			return ErrTopicNotFound
		default:
			return fmt.Errorf("topic metadata error: %w", t.Error)
		}
	}
	return ErrTopicNotFound
}
