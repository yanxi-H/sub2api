package httputil

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/klauspost/compress/zstd"
)

const (
	requestBodyReadInitCap    = 512
	requestBodyReadMaxInitCap = 1 << 20
	jsonUTF8BOMLen            = 3
	// MaxDecompressedBodySize limits the decompressed request body to 64 MB
	// to prevent decompression bomb attacks.
	MaxDecompressedBodySize = int64(64 << 20)
)

// ReadRequestBodyWithPrealloc reads request body with preallocated buffer based
// on content length, transparently decoding any Content-Encoding the upstream
// client used to compress the body (zstd, gzip, deflate).
func ReadRequestBodyWithPrealloc(req *http.Request) ([]byte, error) {
	return readRequestBodyWithPrealloc(req, false)
}

func readRequestBodyWithPrealloc(req *http.Request, rejectDecompressedOverflow bool) ([]byte, error) {
	if req == nil || req.Body == nil {
		return nil, nil
	}

	capHint := requestBodyReadInitCap
	if req.ContentLength > 0 {
		switch {
		case req.ContentLength < int64(requestBodyReadInitCap):
			capHint = requestBodyReadInitCap
		case req.ContentLength > int64(requestBodyReadMaxInitCap):
			capHint = requestBodyReadMaxInitCap
		default:
			capHint = int(req.ContentLength)
		}
	}

	buf := bytes.NewBuffer(make([]byte, 0, capHint))
	if _, err := io.Copy(buf, req.Body); err != nil {
		return nil, err
	}
	raw := buf.Bytes()

	enc := strings.ToLower(strings.TrimSpace(req.Header.Get("Content-Encoding")))
	if enc == "" || enc == "identity" {
		return raw, nil
	}

	decoded, err := decompressRequestBody(enc, raw, MaxDecompressedBodySize, rejectDecompressedOverflow)
	if err != nil {
		return nil, fmt.Errorf("decode Content-Encoding %q: %w", enc, err)
	}

	req.Header.Del("Content-Encoding")
	req.Header.Del("Content-Length")
	req.ContentLength = int64(len(decoded))

	return decoded, nil
}

// ReadLenientJSONRequestBodyWithPrealloc reads a request body and normalizes
// JSON string control bytes before strict validation.
func ReadLenientJSONRequestBodyWithPrealloc(req *http.Request, maxNormalizedBytes int64) ([]byte, error) {
	if maxNormalizedBytes <= 0 {
		maxNormalizedBytes = MaxDecompressedBodySize
	}
	body, err := readRequestBodyWithPreallocLimit(req, maxNormalizedBytes)
	if err != nil {
		return nil, err
	}
	return NormalizeLenientJSONRequestBody(body, maxNormalizedBytes)
}

func readRequestBodyWithPreallocLimit(req *http.Request, maxBodyBytes int64) ([]byte, error) {
	if req == nil || req.Body == nil {
		return nil, nil
	}
	if maxBodyBytes <= 0 {
		return nil, &http.MaxBytesError{Limit: maxBodyBytes}
	}

	raw, err := readAllWithLimit(req.Body, maxBodyBytes)
	if err != nil {
		return nil, err
	}
	enc := strings.ToLower(strings.TrimSpace(req.Header.Get("Content-Encoding")))
	if enc == "" || enc == "identity" {
		return raw, nil
	}

	decodedLimit := maxBodyBytes
	if decodedLimit > MaxDecompressedBodySize {
		decodedLimit = MaxDecompressedBodySize
	}
	decoded, err := decompressRequestBody(enc, raw, decodedLimit, true)
	if err != nil {
		return nil, fmt.Errorf("decode Content-Encoding %q: %w", enc, err)
	}
	req.Header.Del("Content-Encoding")
	req.Header.Del("Content-Length")
	req.ContentLength = int64(len(decoded))
	return decoded, nil
}

func decompressRequestBody(encoding string, raw []byte, maxDecodedBytes int64, rejectOverflow bool) ([]byte, error) {
	readDecoded := func(reader io.Reader) ([]byte, error) {
		if rejectOverflow {
			return readAllWithLimit(reader, maxDecodedBytes)
		}
		return io.ReadAll(io.LimitReader(reader, maxDecodedBytes))
	}
	switch encoding {
	case "zstd":
		dec, err := zstd.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		defer dec.Close()
		return readDecoded(dec)
	case "gzip", "x-gzip":
		gr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		defer func() { _ = gr.Close() }()
		return readDecoded(gr)
	case "deflate":
		zr, err := zlib.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		defer func() { _ = zr.Close() }()
		return readDecoded(zr)
	default:
		return nil, errors.New("unsupported Content-Encoding")
	}
}

func readAllWithLimit(r io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		return nil, &http.MaxBytesError{Limit: limit}
	}
	body, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, &http.MaxBytesError{Limit: limit}
	}
	return body, nil
}

// NormalizeLenientJSONRequestBody escapes raw control bytes that broken
// OpenAI-compatible clients sometimes place inside JSON strings.
func NormalizeLenientJSONRequestBody(body []byte, maxNormalizedBytes int64) ([]byte, error) {
	if maxNormalizedBytes <= 0 {
		maxNormalizedBytes = MaxDecompressedBodySize
	}

	body = trimUTF8BOM(body)
	if len(body) == 0 {
		return body, nil
	}
	if int64(len(body)) > maxNormalizedBytes {
		return nil, &http.MaxBytesError{Limit: maxNormalizedBytes}
	}

	var out []byte
	inString := false
	escaped := false
	for i, b := range body {
		if inString && isJSONControlByte(b) {
			if out == nil {
				capHint := len(body) + 6
				if int64(capHint) > maxNormalizedBytes {
					capHint = int(maxNormalizedBytes)
				}
				out = make([]byte, 0, capHint)
				out = append(out, body[:i]...)
			}
			if int64(len(out)+6) > maxNormalizedBytes {
				return nil, &http.MaxBytesError{Limit: maxNormalizedBytes}
			}
			out = appendJSONUnicodeEscape(out, b)
			escaped = false
			continue
		}

		switch {
		case escaped:
			escaped = false
		case inString && b == '\\':
			escaped = true
		case b == '"':
			inString = !inString
		}

		if out != nil {
			if int64(len(out)+1) > maxNormalizedBytes {
				return nil, &http.MaxBytesError{Limit: maxNormalizedBytes}
			}
			out = append(out, b)
		}
	}
	if out != nil {
		return out, nil
	}
	return body, nil
}

func trimUTF8BOM(body []byte) []byte {
	if len(body) >= jsonUTF8BOMLen && body[0] == 0xef && body[1] == 0xbb && body[2] == 0xbf {
		return body[jsonUTF8BOMLen:]
	}
	return body
}

func isJSONControlByte(b byte) bool {
	return b < 0x20 || b == 0x7f
}

func appendJSONUnicodeEscape(dst []byte, b byte) []byte {
	const hex = "0123456789abcdef"
	return append(dst, '\\', 'u', '0', '0', hex[b>>4], hex[b&0x0f])
}
