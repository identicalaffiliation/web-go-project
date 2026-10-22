//go:build integration

package integration

import (
	"testing"
	"uuid"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/suite"
)

func TestConsumerSuite(t *testing.T) {
	suite.Run(t, new(ConsumerSuite))
}

func (s *ConsumerSuite) Test_CreatesStockAndCommits() {
	s.createTopic(1)
	s.startConsumers(1)

	p1, p2 := newProduct(10), newProduct(5)
	s.sendProducts(p1, p2)

	s.waitCommitted(1, 2)

	qty, ok := s.stockQuantity(p1.ID)
	s.Require().True(ok)
	s.Equal(int16(10), qty)

	qty, ok = s.stockQuantity(p2.ID)
	s.Require().True(ok)
	s.Equal(int16(5), qty)
}

// Дубль события (ретрай продюсера/outbox) не должен создавать вторую строку, перезаписывать остаток
// и застревать незакоммиченным.
func (s *ConsumerSuite) Test_DuplicateMessage_IsCommittedAndDoesNotOverwrite() {
	s.createTopic(1)
	s.startConsumers(1)

	p := newProduct(10)
	duplicate := p
	duplicate.Quantity = 50
	s.sendProducts(p, duplicate)

	s.waitCommitted(1, 2)

	s.Equal(1, s.stockCount())
	qty, ok := s.stockQuantity(p.ID)
	s.Require().True(ok)
	s.Equal(int16(10), qty)
}

// Событие пришло повторно, когда по товару уже были резервы - остаток не откатывается к исходному.
func (s *ConsumerSuite) Test_ExistingStock_IsNotOverwritten() {
	p := newProduct(10)
	s.insertStock(p.ID, 3)

	s.createTopic(1)
	s.startConsumers(1)
	s.sendProducts(p)

	s.waitCommitted(1, 1)

	qty, ok := s.stockQuantity(p.ID)
	s.Require().True(ok)
	s.Equal(int16(3), qty)
}

func (s *ConsumerSuite) Test_InvalidJSON_IsCommittedAndSkipped() {
	s.createTopic(1)
	s.startConsumers(1)

	p := newProduct(7)
	s.sendRaw(kafka.Message{Value: []byte("not json")})
	s.sendProducts(p)

	s.waitCommitted(1, 2)

	s.Equal(1, s.stockCount())
	qty, ok := s.stockQuantity(p.ID)
	s.Require().True(ok)
	s.Equal(int16(7), qty)
}

// Нарушение CHECK (quantity < 100) - сообщение битое, ретраить бессмысленно: коммитим и пропускаем.
// Сообщение последнее в партиции, чтобы коммит следующего не "перепрыгнул" через него.
func (s *ConsumerSuite) Test_InvalidQuantity_IsCommittedAndSkipped() {
	s.createTopic(1)
	s.startConsumers(1)

	s.sendProducts(newProduct(150))

	s.waitCommitted(1, 1)

	s.Equal(0, s.stockCount())
}

// Несколько воркеров в одной группе делят партиции: каждое сообщение обработано ровно один раз.
func (s *ConsumerSuite) Test_MultipleWorkers_ProcessAllPartitions() {
	const (
		partitions = 3
		products   = 30
	)

	s.createTopic(partitions)
	s.startConsumers(partitions)

	sent := make(map[uuid.UUID]int16, products)
	for i := range products {
		p := newProduct(int16(i%90 + 1))
		sent[p.ID] = p.Quantity
		s.sendProducts(p)
	}

	s.waitCommitted(partitions, products)

	s.Equal(products, s.stockCount())
	for id, want := range sent {
		qty, ok := s.stockQuantity(id)
		s.Require().True(ok)
		s.Equal(want, qty)
	}
}
