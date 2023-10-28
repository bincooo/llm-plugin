package util

import (
	"github.com/bincooo/llm-plugin/internal/vars"
	nano "github.com/fumiama/NanoBot"
	"github.com/sirupsen/logrus"
	"sync"
	"time"
)

const (
	waitTimeout = 3 * time.Second
)

func NewGifTimer(ctx *nano.Ctx, enable bool) *GifTimer {
	d := GifTimer{t: time.Now().Add(waitTimeout), closed: false, ctx: ctx}
	if enable {
		go d.run()
	}
	return &d
}

type GifTimer struct {
	mu        sync.Mutex
	t         time.Time
	next      bool
	closed    bool
	ctx       *nano.Ctx
	messageId string
}

func (d *GifTimer) Refill() {
	d.t = time.Now().Add(waitTimeout)
	// 需要执行
	d.next = true
}

func (d *GifTimer) Release() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
	d.next = false
}

func (d *GifTimer) send() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return
	}
	if d.messageId != "" {
		if err := Retry(1, func() error {
			return d.ctx.DeleteMessageInChannel(d.ctx.Message.ChannelID, d.messageId, true)
		}); err != nil {
			logrus.Error(err)
		}
	}
	time.Sleep(500 * time.Millisecond)
	// messageId := d.ctx.Send(message.ImageBytes(vars.Loading))
	var message *nano.Message
	if err := Retry(1, func() error {
		// m, e := d.ctx.SendPlainMessage(false, "...")
		m, e := d.ctx.SendImageBytes(vars.Loading, false)
		if e == nil {
			message = m
		}
		return e
	}); err != nil {
		logrus.Error(err)
	} else {
		d.messageId = message.ID
	}
}

func (d *GifTimer) run() {
	for {
		if d.closed {
			if err := Retry(1, func() error {
				return d.ctx.DeleteMessageInChannel(d.ctx.Message.ChannelID, d.messageId, true)
			}); err != nil {
				logrus.Error(err)
			}
			return
		}

		if d.next && time.Now().After(d.t) {
			d.send()
			d.next = false
		}
		time.Sleep(800 * time.Millisecond)
	}
}
