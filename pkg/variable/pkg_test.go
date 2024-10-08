package variable

import (
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/nite-coder/bifrost/pkg/config"
	"github.com/nite-coder/blackbear/pkg/cast"
	"github.com/stretchr/testify/assert"
)

func TestGetDirective(t *testing.T) {
	hzCtx := app.NewContext(0)

	hzCtx.Set(config.SERVER_ID, "serverA")
	hzCtx.Request.Header.SetUserAgentBytes([]byte("my_user_agent"))
	hzCtx.Set(config.TRACE_ID, "trace_id")
	hzCtx.SetClientIPFunc(func(ctx *app.RequestContext) string {
		return "127.0.0.1"
	})
	hzCtx.Request.SetMethod("GET")
	hzCtx.Request.URI().SetPath("/foo")

	val, found := Get("$client_ip", hzCtx)
	assert.True(t, found)
	assert.Equal(t, "127.0.0.1", val)

	val, found = Get(config.SERVER_ID, hzCtx)
	assert.True(t, found)
	assert.Equal(t, "serverA", val)

	val, found = Get(config.REQUEST_PATH, hzCtx)
	assert.True(t, found)
	assert.Equal(t, "/foo", val)

	val, found = Get(config.UserAgent, hzCtx)
	userAgent, _ := cast.ToString(val)
	assert.True(t, found)
	assert.Equal(t, "my_user_agent", userAgent)

	val, found = Get(config.TRACE_ID, hzCtx)
	traceID, _ := cast.ToString(val)
	assert.True(t, found)
	assert.Equal(t, "trace_id", traceID)

	val, found = Get(config.DURATION, hzCtx)
	assert.False(t, found)
	assert.Nil(t, val)

	val, found = Get("", hzCtx)
	assert.False(t, found)
	assert.Nil(t, val)

	val, found = Get("aaa", nil)
	assert.False(t, found)
	assert.Nil(t, val)
}

func TestGetVariable(t *testing.T) {
	hzCtx := app.NewContext(0)

	hzCtx.Set("uid", "123456")
	hzCtx.Request.SetMethod("GET")
	hzCtx.Request.URI().SetPath("/foo")

	uid, found := Get("$var.uid", hzCtx)
	assert.True(t, found)
	assert.Equal(t, "123456", uid)

	val, found := Get("$var.aaa", nil)
	assert.False(t, found)
	assert.Nil(t, val)
}
