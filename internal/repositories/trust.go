package repositories

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"

	"sysentra/internal/runner"
)

const maxRepodataSize = 64 << 20

type KeyIdentity struct {
	Signer      string
	Type        string
	Bits        string
	Fingerprint string
}

type TrustInspector struct {
	runner runner.Runner
	client *http.Client
}

func NewTrustInspector(commandRunner runner.Runner, client *http.Client) *TrustInspector {
	return &TrustInspector{runner: commandRunner, client: client}
}

func (i *TrustInspector) Inspect(ctx context.Context, repositoryURL string) (KeyIdentity, error) {
	parsed, err := url.Parse(repositoryURL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return KeyIdentity{}, errors.New("a valid HTTPS repository URL is required")
	}
	architecture := runtime.GOARCH
	if architecture == "amd64" {
		architecture = "x86_64"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(repositoryURL, "/")+"/"+architecture+"-repodata", nil)
	if err != nil {
		return KeyIdentity{}, err
	}
	response, err := i.client.Do(request)
	if err != nil {
		return KeyIdentity{}, err
	}
	defer response.Body.Close()
	if response.Request.URL.Scheme != "https" {
		return KeyIdentity{}, errors.New("repository redirected to a non-HTTPS URL")
	}
	if response.StatusCode != http.StatusOK {
		return KeyIdentity{}, fmt.Errorf("repository returned HTTP %d", response.StatusCode)
	}
	temporary, err := os.CreateTemp("", "sysentra-repodata-*.tar.zst")
	if err != nil {
		return KeyIdentity{}, err
	}
	path := temporary.Name()
	defer os.Remove(path)
	written, err := io.Copy(temporary, io.LimitReader(response.Body, maxRepodataSize+1))
	closeErr := temporary.Close()
	if err != nil || closeErr != nil {
		return KeyIdentity{}, errors.New("save repository metadata")
	}
	if written > maxRepodataSize {
		return KeyIdentity{}, errors.New("repository metadata exceeds 64 MiB")
	}
	result, err := i.runner.Run(ctx, "/usr/bin/tar", "--zstd", "-xOf", path, "index-meta.plist")
	if err != nil {
		return KeyIdentity{}, fmt.Errorf("read repository signing metadata: %w", err)
	}
	return parseKeyIdentity(result.Stdout)
}

func parseKeyIdentity(contents []byte) (KeyIdentity, error) {
	decoder := xml.NewDecoder(strings.NewReader(string(contents)))
	values := make(map[string]string)
	key := ""
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return KeyIdentity{}, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "key":
			if err := decoder.DecodeElement(&key, &start); err != nil {
				return KeyIdentity{}, err
			}
		case "string", "integer", "data":
			var value string
			if err := decoder.DecodeElement(&value, &start); err != nil {
				return KeyIdentity{}, err
			}
			if key != "" {
				values[key] = strings.TrimSpace(value)
				key = ""
			}
		}
	}
	keyPEM, err := base64.StdEncoding.DecodeString(values["public-key"])
	if err != nil {
		return KeyIdentity{}, errors.New("repository public key is invalid")
	}
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return KeyIdentity{}, errors.New("repository public key is missing")
	}
	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil || publicKey == nil {
		return KeyIdentity{}, errors.New("repository public key cannot be parsed")
	}
	digest := sha256.Sum256(block.Bytes)
	fingerprint := strings.ToUpper(hex.EncodeToString(digest[:]))
	groups := make([]string, 0, len(fingerprint)/2)
	for index := 0; index < len(fingerprint); index += 2 {
		groups = append(groups, fingerprint[index:index+2])
	}
	if values["signature-by"] == "" || values["signature-type"] == "" {
		return KeyIdentity{}, errors.New("repository signing identity is incomplete")
	}
	return KeyIdentity{Signer: values["signature-by"], Type: values["signature-type"], Bits: values["public-key-size"], Fingerprint: strings.Join(groups, ":")}, nil
}
