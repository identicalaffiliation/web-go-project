// Команда keygen генерирует пару RSA-ключей для подписи и проверки JWT (RS256).
//
// Формат файлов совпадает с тем, что читает pkg/jwt: приватный ключ в PKCS8,
// публичный в PKIX. Запускается через `make keys` из корня репозитория.
package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	privateKeyFile = "private.pem"
	publicKeyFile  = "public.pem"

	// Значение для AUTH_JWT_ISSUER — только предложение: оно должно совпадать
	// с iss, который проверяет validator. Согласовать с командой.
	suggestedIssuer = "auth-service"
)

var errKeysExist = errors.New("keys already exist")

func main() {
	out := flag.String("out", "../../secrets/jwt", "directory for private.pem and public.pem")
	bits := flag.Int("bits", 2048, "RSA key size in bits")
	force := flag.Bool("force", false, "overwrite existing keys")
	flag.Parse()

	if err := run(os.Stdout, *out, *bits, *force); err != nil {
		fmt.Fprintln(os.Stderr, "keygen:", err)
		os.Exit(1)
	}
}

func run(stdout io.Writer, outDir string, bits int, force bool) error {
	absDir, err := filepath.Abs(outDir)
	if err != nil {
		return fmt.Errorf("resolve output dir: %w", err)
	}
	privPath := filepath.Join(absDir, privateKeyFile)
	pubPath := filepath.Join(absDir, publicKeyFile)

	if !force {
		existing, err := existingKeyFiles(privPath, pubPath)
		if err != nil {
			return fmt.Errorf("check existing keys: %w", err)
		}
		if len(existing) > 0 {
			return fmt.Errorf("%w: %v (use -force to overwrite)", errKeysExist, existing)
		}
	}

	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return fmt.Errorf("generate RSA key: %w", err)
	}

	privPEM, pubPEM, kid, err := encodeKeys(key)
	if err != nil {
		return err
	}

	// 0o700: папка с приватным ключом не должна быть доступна другим пользователям.
	if err := os.MkdirAll(absDir, 0o700); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	if err := writeKeyFile(privPath, privPEM, 0o600, force); err != nil {
		return err
	}
	if err := writeKeyFile(pubPath, pubPEM, 0o644, force); err != nil {
		return err
	}

	if err := printSummary(stdout, privPath, pubPath, kid, force); err != nil {
		return fmt.Errorf("print summary: %w", err)
	}
	return nil
}

func existingKeyFiles(privPath, pubPath string) ([]string, error) {
	var existing []string
	for _, path := range []string{privPath, pubPath} {
		_, err := os.Stat(path)
		switch {
		case err == nil:
			existing = append(existing, path)
		case errors.Is(err, fs.ErrNotExist):
		default:
			return nil, fmt.Errorf("stat key file %s: %w", path, err)
		}
	}

	return existing, nil
}

func encodeKeys(key *rsa.PrivateKey) (privPEM, pubPEM []byte, kid string, err error) {
	privDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, nil, "", fmt.Errorf("marshal private key: %w", err)
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return nil, nil, "", fmt.Errorf("marshal public key: %w", err)
	}

	sum := sha256.Sum256(pubDER)
	kid = hex.EncodeToString(sum[:8])

	privPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privDER})
	pubPEM = pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	return privPEM, pubPEM, kid, nil
}

func writeKeyFile(path string, data []byte, perm os.FileMode, force bool) error {
	if force {
		return replaceFile(path, data, perm)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("%w: %s (use -force to overwrite)", errKeysExist, path)
		}
		return fmt.Errorf("create %s: %w", path, err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}
	return nil
}

func replaceFile(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", path, err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("write %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close %s: %w", tmpName, err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		cleanup()
		return fmt.Errorf("chmod %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

// printSummary собирает текст целиком и пишет его одним вызовом: так ошибку
// записи достаточно проверить один раз, а не после каждой строки.
func printSummary(w io.Writer, privPath, pubPath, kid string, force bool) error {
	var b strings.Builder
	b.WriteString("Keys written:\n")
	b.WriteString("  private: " + privPath + "\n")
	b.WriteString("  public:  " + pubPath + "\n")
	b.WriteString("\n")
	b.WriteString("Environment variables for auth:\n")
	b.WriteString("AUTH_JWT_PRIVATE_KEY_PATH=" + privPath + "\n")
	b.WriteString("AUTH_JWT_PUBLIC_KEY_PATH=" + pubPath + "\n")
	b.WriteString("AUTH_JWT_KEY_ID=" + kid + "\n")
	b.WriteString("AUTH_JWT_ISSUER=" + suggestedIssuer + "\n")
	b.WriteString("\n")
	b.WriteString("validator needs only public.pem; never commit or share private.pem.\n")
	if force {
		b.WriteString("WARNING: keys were replaced. All issued tokens are now invalid; restart auth and validator.\n")
	}

	_, err := io.WriteString(w, b.String())
	return err
}
