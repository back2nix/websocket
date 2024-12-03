// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/savsgio/gotils/strconv"
	"github.com/valyala/fasthttp"
)

var strPermessageDeflate = []byte("permessage-deflate")

var poolWriteBuffer = sync.Pool{
	New: func() interface{} {
		return new(writePoolData)
	},
}

// FastHTTPHandler receives a websocket connection after the handshake has been
// completed. This must be provided.
type FastHTTPHandler func(*Conn)

// FastHTTPUpgrader specifies parameters for upgrading an HTTP connection to a
// WebSocket connection.
type FastHTTPUpgrader struct {
	WriteBufferPool   BufferPool
	Error             func(ctx *fasthttp.RequestCtx, status int, reason error)
	CheckOrigin       func(ctx *fasthttp.RequestCtx) bool
	Subprotocols      []string
	HandshakeTimeout  time.Duration
	ReadBufferSize    int
	WriteBufferSize   int
	EnableCompression bool
}

func (u *FastHTTPUpgrader) responseError(ctx *fasthttp.RequestCtx, status int, reason string) error {
	err := HandshakeError{reason}
	if u.Error != nil {
		u.Error(ctx, status, err)
	} else {
		ctx.Response.Header.Set("Sec-Websocket-Version", "13")
		ctx.Error(fasthttp.StatusMessage(status), status)
	}

	return err
}

func (u *FastHTTPUpgrader) selectSubprotocol(ctx *fasthttp.RequestCtx) []byte {
	if u.Subprotocols != nil {
		clientProtocols := parseDataHeader(ctx.Request.Header.Peek("Sec-Websocket-Protocol"))

		for _, serverProtocol := range u.Subprotocols {
			for _, clientProtocol := range clientProtocols {
				if strconv.B2S(clientProtocol) == serverProtocol {
					return clientProtocol
				}
			}
		}
	} else if ctx.Response.Header.Len() > 0 {
		return ctx.Response.Header.Peek("Sec-Websocket-Protocol")
	}

	return nil
}

func (u *FastHTTPUpgrader) isCompressionEnable(ctx *fasthttp.RequestCtx) bool {
	extensions := parseDataHeader(ctx.Request.Header.Peek("Sec-WebSocket-Extensions"))

	// Negotiate PMCE
	if u.EnableCompression {
		for _, ext := range extensions {
			if bytes.HasPrefix(ext, strPermessageDeflate) {
				return true
			}
		}
	}

	return false
}

// Upgrade upgrades the HTTP server connection to the WebSocket protocol.
//
// The responseHeader is included in the response to the client's upgrade
// request. Use the responseHeader to specify cookies (Set-Cookie) and the
// application negotiated subprotocol (Sec-WebSocket-Protocol).
//
// If the upgrade fails, then Upgrade replies to the client with an HTTP error
// response.
func (u *FastHTTPUpgrader) Upgrade(ctx *fasthttp.RequestCtx, handler FastHTTPHandler) error {
	if !ctx.IsGet() {
		return u.responseError(ctx, fasthttp.StatusMethodNotAllowed, fmt.Sprintf("%s request method is not GET", badHandshake))
	}

	if !tokenContainsValue(strconv.B2S(ctx.Request.Header.Peek("Connection")), "Upgrade") {
		return u.responseError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("%s 'upgrade' token not found in 'Connection' header", badHandshake))
	}

	if !tokenContainsValue(strconv.B2S(ctx.Request.Header.Peek("Upgrade")), "Websocket") {
		return u.responseError(ctx, fasthttp.StatusBadRequest, fmt.Sprintf("%s 'websocket' token not found in 'Upgrade' header", badHandshake))
	}

	if !tokenContainsValue(strconv.B2S(ctx.Request.Header.Peek("Sec-Websocket-Version")), "13") {
		return u.responseError(ctx, fasthttp.StatusBadRequest, "websocket: unsupported version: 13 not found in 'Sec-Websocket-Version' header")
	}

	if len(ctx.Response.Header.Peek("Sec-Websocket-Extensions")) > 0 {
		return u.responseError(ctx, fasthttp.StatusInternalServerError, "websocket: application specific 'Sec-WebSocket-Extensions' headers are unsupported")
	}

	checkOrigin := u.CheckOrigin
	if checkOrigin == nil {
		checkOrigin = fastHTTPcheckSameOrigin
	}
	if !checkOrigin(ctx) {
		return u.responseError(ctx, fasthttp.StatusForbidden, "websocket: request origin not allowed by FastHTTPUpgrader.CheckOrigin")
	}

	challengeKey := ctx.Request.Header.Peek("Sec-Websocket-Key")
	if len(challengeKey) == 0 {
		return u.responseError(ctx, fasthttp.StatusBadRequest, "websocket: not a websocket handshake: `Sec-WebSocket-Key' header is missing or blank")
	}

	subprotocol := u.selectSubprotocol(ctx)
	compress := u.isCompressionEnable(ctx)

	ctx.SetStatusCode(fasthttp.StatusSwitchingProtocols)
	ctx.Response.Header.Set("Upgrade", "websocket")
	ctx.Response.Header.Set("Connection", "Upgrade")
	ctx.Response.Header.Set("Sec-WebSocket-Accept", computeAcceptKeyBytes(challengeKey))
	if compress {
		ctx.Response.Header.Set("Sec-WebSocket-Extensions", "permessage-deflate; server_no_context_takeover; client_no_context_takeover")
	}
	if subprotocol != nil {
		ctx.Response.Header.SetBytesV("Sec-WebSocket-Protocol", subprotocol)
	}

	ctx.Hijack(func(netConn net.Conn) {
		// var br *bufio.Reader  // Always nil
		writeBuf := poolWriteBuffer.Get().(*writePoolData)

		c := newConn(netConn, true, u.ReadBufferSize, u.WriteBufferSize, u.WriteBufferPool, nil, writeBuf.buf)
		if subprotocol != nil {
			c.subprotocol = strconv.B2S(subprotocol)
		}

		if compress {
			c.newCompressionWriter = compressNoContextTakeover
			c.newDecompressionReader = decompressNoContextTakeover
		}

		// Clear deadlines set by HTTP server.
		_ = netConn.SetDeadline(time.Time{})

		handler(c)

		writeBuf.buf = writeBuf.buf[0:0]
		poolWriteBuffer.Put(writeBuf)
	})

	return nil
}

// fastHTTPcheckSameOrigin returns true if the origin is not set or is equal to the request host.
func fastHTTPcheckSameOrigin(ctx *fasthttp.RequestCtx) bool {
	origin := ctx.Request.Header.Peek("Origin")
	if len(origin) == 0 {
		return true
	}
	u, err := url.Parse(strconv.B2S(origin))
	if err != nil {
		return false
	}
	return equalASCIIFold(u.Host, strconv.B2S(ctx.Host()))
}

// FastHTTPIsWebSocketUpgrade returns true if the client requested upgrade to the
// WebSocket protocol.
func FastHTTPIsWebSocketUpgrade(ctx *fasthttp.RequestCtx) bool {
	return tokenContainsValue(strconv.B2S(ctx.Request.Header.Peek("Connection")), "Upgrade") &&
		tokenContainsValue(strconv.B2S(ctx.Request.Header.Peek("Upgrade")), "Websocket")
}
