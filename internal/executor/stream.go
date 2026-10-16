package executor

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"strings"
)

// StreamChunk 表示单次流式响应块
type StreamChunk struct {
	Payload []byte
	Err     error
	Usage   *UsageInfo
}

// ReadSSEStream 从上游 HTTP Response Body 读取并解析 SSE 事件流
func ReadSSEStream(ctx context.Context, body io.ReadCloser) <-chan StreamChunk {
	out := make(chan StreamChunk, 32)

	go func() {
		defer close(out)
		defer body.Close()

		reader := bufio.NewReader(body)
		var eventBuf bytes.Buffer

		for {
			select {
			case <-ctx.Done():
				out <- StreamChunk{Err: ctx.Err()}
				return
			default:
			}

			line, err := reader.ReadBytes('\n')
			if err != nil {
				if err != io.EOF {
					out <- StreamChunk{Err: err}
				} else if eventBuf.Len() > 0 {
					chunkData := eventBuf.Bytes()
					usage := ParseTerminalUsage(chunkData)
					out <- StreamChunk{Payload: chunkData, Usage: usage}
				}
				return
			}

			eventBuf.Write(line)

			// SSE 规范：双换行表示一个完整事件结束
			if bytes.Equal(line, []byte("\n")) || bytes.Equal(line, []byte("\r\n")) {
				chunkBytes := make([]byte, eventBuf.Len())
				copy(chunkBytes, eventBuf.Bytes())
				eventBuf.Reset()

				trimmed := strings.TrimSpace(string(chunkBytes))
				if trimmed == "" {
					continue
				}

				usage := ParseTerminalUsage(chunkBytes)
				out <- StreamChunk{
					Payload: chunkBytes,
					Usage:   usage,
				}
			}
		}
	}()

	return out
}
