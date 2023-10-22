package util

import (
	"github.com/bincooo/llm-plugin/internal/vars"
	"github.com/wdvxdr1123/ZeroBot/message"
	"sync"
	"time"

	zero "github.com/wdvxdr1123/ZeroBot"
)

const (
	waitTimeout = 3 * time.Second
)

func NewGifTimer(ctx *zero.Ctx, enable bool) *GifTimer {
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
	ctx       *zero.Ctx
	messageId *message.MessageID
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
	if d.messageId != nil {
		d.ctx.DeleteMessage(*d.messageId)
	}
	time.Sleep(500 * time.Millisecond)
	messageId := d.ctx.Send(message.ImageBytes(vars.Loading))
	d.messageId = &messageId
}

func (d *GifTimer) run() {
	for {
		if d.closed {
			if d.messageId != nil {
				d.ctx.DeleteMessage(*d.messageId)
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
