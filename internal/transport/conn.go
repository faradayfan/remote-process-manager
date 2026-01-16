package transport

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/faradayfan/remote-process-manager/internal/protocol"
)

type Conn struct {
	c net.Conn
	r *bufio.Reader
	w *bufio.Writer
}

func NewConn(c net.Conn) *Conn {
	return &Conn{
		c: c,
		r: bufio.NewReader(c),
		w: bufio.NewWriter(c),
	}
}

func (c *Conn) Close() error {
	return c.c.Close()
}

func (c *Conn) SetDeadline(t time.Time) error {
	return c.c.SetDeadline(t)
}

func (c *Conn) Send(msg protocol.Message) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// newline framed
	if _, err := c.w.Write(append(b, '\n')); err != nil {
		return err
	}
	return c.w.Flush()
}

func (c *Conn) Recv() (protocol.Message, error) {
	line, err := c.r.ReadBytes('\n')
	if err != nil {
		if err == io.EOF {
			return protocol.Message{}, io.EOF
		}
		return protocol.Message{}, err
	}

	var msg protocol.Message
	if err := json.Unmarshal(line, &msg); err != nil {
		return protocol.Message{}, fmt.Errorf("invalid json: %w", err)
	}
	return msg, nil
}
