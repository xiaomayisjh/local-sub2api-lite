//go:build unit

package service

import (
	"errors"
	"io"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// transport-level 错误必须被包成 UpstreamFailoverError，让 handler 层的 failover_loop 接管。
// 历史 bug：transport 错误直接 c.JSON(502)+return，导致：
//  1. handler 收到的是普通 error，errors.As(&failoverErr) 不匹配 → 不换号
//  2. c.Writer 已被污染，即使能匹配也会被 "stream already started" 保护逻辑禁止 failover
func TestWrapTransportErrorAsFailover_RecognizesCommonNetworkErrors(t *testing.T) {
	cases := []struct {
		name     string
		err      error
		wantPart string
	}{
		{name: "EOF", err: io.EOF, wantPart: "EOF"},
		{name: "unexpected_EOF", err: io.ErrUnexpectedEOF, wantPart: "unexpected EOF"},
		{name: "connection_reset", err: syscall.ECONNRESET, wantPart: "connection reset"},
		{name: "connection_refused", err: syscall.ECONNREFUSED, wantPart: "connection refused"},
		{name: "broken_pipe", err: syscall.EPIPE, wantPart: "broken pipe"},
		{name: "timeout", err: syscall.ETIMEDOUT, wantPart: "timed out"},
		{name: "generic", err: errors.New("dial tcp: lookup api.example.com: no such host"), wantPart: "upstream connection error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fe := wrapTransportErrorAsFailover(tc.err)
			require.NotNil(t, fe)
			require.Equal(t, 0, fe.StatusCode, "StatusCode=0 表示 transport 层错误，没拿到响应")
			require.False(t, fe.RetryableOnSameAccount, "transport 错误不重试同账号，直接换号")
			require.NotEmpty(t, fe.ResponseBody, "兜底响应体必须有内容，failover 耗尽时由 handler 写给客户端")

			// 验证 ResponseBody 是合法 Anthropic 风格的 JSON 错误
			body := string(fe.ResponseBody)
			require.Equal(t, "error", gjson.Get(body, "type").String())
			require.Equal(t, "upstream_error", gjson.Get(body, "error.type").String())
			msg := gjson.Get(body, "error.message").String()
			require.True(t, strings.Contains(msg, tc.wantPart),
				"error message %q should contain %q (sanitized form of the original err)", msg, tc.wantPart)
		})
	}
}

func TestWrapTransportErrorAsFailoverOpenAI_UsesOpenAIErrorEnvelope(t *testing.T) {
	fe := wrapTransportErrorAsFailoverOpenAI(io.EOF)
	require.NotNil(t, fe)
	require.Equal(t, 0, fe.StatusCode)
	require.NotEmpty(t, fe.ResponseBody)
	body := string(fe.ResponseBody)
	// OpenAI 风格：没有外层 "type":"error"，只有 "error":{...}
	require.False(t, gjson.Get(body, "type").Exists(), "OpenAI 错误格式不带外层 type")
	require.Equal(t, "upstream_error", gjson.Get(body, "error.type").String())
	require.Contains(t, gjson.Get(body, "error.message").String(), "EOF")
}

// 验证 errors.As 能从包装后的错误中拿到 *UpstreamFailoverError——
// handler 层的 failover_loop 就是靠这个判断"该 failover 还是直接失败"。
func TestWrapTransportErrorAsFailover_ErrorAsRecoverable(t *testing.T) {
	wrapped := wrapTransportErrorAsFailover(io.EOF)
	var got *UpstreamFailoverError
	require.True(t, errors.As(wrapped, &got), "wrapped error must be matchable via errors.As")
	require.Same(t, wrapped, got)
}
