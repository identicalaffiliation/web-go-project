package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/identicalaffiliation/web-go-project/auth/internal/domain"
	authjwt "github.com/identicalaffiliation/web-go-project/auth/pkg/jwt"
)

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}

func TestRun_GeneratesKeysUsableByJWTPackage(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer

	if err := run(&out, dir, 2048, false); err != nil {
		t.Fatalf("run: %v", err)
	}

	priv, err := authjwt.ParsePrivateKeyPEM(readFile(t, filepath.Join(dir, privateKeyFile)))
	if err != nil {
		t.Fatalf("ParsePrivateKeyPEM: %v", err)
	}
	pub, err := authjwt.ParsePublicKeyPEM(readFile(t, filepath.Join(dir, publicKeyFile)))
	if err != nil {
		t.Fatalf("ParsePublicKeyPEM: %v", err)
	}

	// Токен, подписанный сгенерированным приватным ключом, должен проверяться
	// сгенерированным публичным: так мы убеждаемся, что это именно пара.
	issuer := authjwt.NewIssuer(priv, "kid", suggestedIssuer, time.Minute)
	token, _, err := issuer.Issue(domain.Claims{UserID: uuid.MustParse("019a0000-0000-7000-8000-000000000001")})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := authjwt.NewVerifier(pub, suggestedIssuer).Verify(token); err != nil {
		t.Fatalf("Verify: %v", err)
	}

	summary := out.String()
	for _, want := range []string{"AUTH_JWT_PRIVATE_KEY_PATH=", "AUTH_JWT_PUBLIC_KEY_PATH=", "AUTH_JWT_KEY_ID=", "AUTH_JWT_ISSUER="} {
		if !strings.Contains(summary, want) {
			t.Errorf("summary must contain %q, got:\n%s", want, summary)
		}
	}
	if strings.Contains(summary, "PRIVATE KEY-----") {
		t.Error("summary must not contain key material")
	}
}

func TestRun_DoesNotOverwriteWithoutForce(t *testing.T) {
	dir := t.TempDir()
	if err := run(&bytes.Buffer{}, dir, 2048, false); err != nil {
		t.Fatalf("first run: %v", err)
	}
	privBefore := readFile(t, filepath.Join(dir, privateKeyFile))
	pubBefore := readFile(t, filepath.Join(dir, publicKeyFile))

	err := run(&bytes.Buffer{}, dir, 2048, false)
	if !errors.Is(err, errKeysExist) {
		t.Fatalf("got %v, want errKeysExist", err)
	}

	if !bytes.Equal(privBefore, readFile(t, filepath.Join(dir, privateKeyFile))) {
		t.Error("private key was overwritten")
	}
	if !bytes.Equal(pubBefore, readFile(t, filepath.Join(dir, publicKeyFile))) {
		t.Error("public key was overwritten")
	}
}

func TestRun_ForceReplacesKeys(t *testing.T) {
	dir := t.TempDir()
	if err := run(&bytes.Buffer{}, dir, 2048, false); err != nil {
		t.Fatalf("first run: %v", err)
	}
	privBefore := readFile(t, filepath.Join(dir, privateKeyFile))

	var out bytes.Buffer
	if err := run(&out, dir, 2048, true); err != nil {
		t.Fatalf("force run: %v", err)
	}

	if bytes.Equal(privBefore, readFile(t, filepath.Join(dir, privateKeyFile))) {
		t.Error("private key must change with -force")
	}
	if !strings.Contains(out.String(), "WARNING") {
		t.Error("force run must warn that issued tokens become invalid")
	}

	// Временные файлы не должны оставаться в папке с ключами.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("want exactly 2 files in %s, got %d", dir, len(entries))
	}
}
