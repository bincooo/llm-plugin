package utils

import (
	"github.com/bincooo/llm-plugin/internal/repo/store"
	"github.com/bincooo/llm-plugin/vars"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"github.com/wdvxdr1123/ZeroBot/message"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	waitTimeout = 3 * time.Second
)

func StringToMessageSegment(uid, msg string) []message.MessageSegment {
	msg = removeUrlBlock(msg)

	// 转换消息对象Chain
	msg = strings.ReplaceAll(msg, "&#91;", "[")
	msg = strings.ReplaceAll(msg, "&#93;", "]")

	compileRegex := regexp.MustCompile(`\[@[^]]+]`)
	matches := compileRegex.FindAllStringSubmatch(msg, -1)
	logrus.Info("StringToMessageSegment CQ:At:: ", matches)
	pos := 0
	online := store.GetOnline(uid)
	var slice []message.MessageSegment

	for _, mat := range matches {
		if len(mat) == 0 {
			continue
		}

		qq := strings.TrimPrefix(strings.TrimSuffix(mat[0], "]"), "[@")
		qq = strings.TrimSpace(qq)
		if qq == "" {
			continue
		}

		index := strings.Index(msg, mat[0])
		if index < 0 || index <= pos {
			continue
		}

		contain := ContainFor(online, func(item store.OKv) bool {
			if item.Name == qq {
				qq = item.Id
			}
			return item.Id == qq
		})

		if contain {
			slice = append(slice, message.Text(msg[pos:index]))
			pos = index + len(qq) + 3
			at, err := strconv.ParseInt(strings.TrimSpace(qq), 10, 64)
			if err != nil {
				continue
			}
			slice = append(slice, message.At(at))
		}
	}

	if len(msg)-1 > pos {
		slice = append(slice, message.Text(msg[pos:]))
	}

	return slice
}

func removeUrlBlock(msg string) string {
	regexCompile := regexp.MustCompile(`\[\^[0-9]\^]`)
	msg = regexCompile.ReplaceAllString(msg, "")
	regexCompile = regexp.MustCompile(`\[\^[0-9]\^\^`)
	return regexCompile.ReplaceAllString(msg, "...")
}

// 判断切片是否包含子元素
func Contains[T comparable](slice []T, t T) bool {
	if len(slice) == 0 {
		return false
	}

	return ContainFor(slice, func(item T) bool {
		return item == t
	})
}

// 判断切片是否包含子元素， condition：自定义判断规则
func ContainFor[T comparable](slice []T, condition func(item T) bool) bool {
	if len(slice) == 0 {
		return false
	}

	for idx := 0; idx < len(slice); idx++ {
		if condition(slice[idx]) {
			return true
		}
	}
	return false
}

// ========================================

func NewDelay(ctx *zero.Ctx, enable bool) *Delay {
	if !enable {
		return &Delay{}
	}
	d := Delay{t: time.Now().Add(waitTimeout), closed: false, ctx: ctx}
	go d.run()
	return &d
}

// 续时器
type Delay struct {
	mu        sync.Mutex
	t         time.Time
	next      bool
	closed    bool
	ctx       *zero.Ctx
	messageId *message.MessageID
}

func (d *Delay) Defer() {
	d.t = time.Now().Add(waitTimeout)
	// 需要执行
	d.next = true
}

func (d *Delay) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
	d.next = false
}

func (d *Delay) Send() {
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

func (d *Delay) run() {
	for {
		if d.closed {
			if d.messageId != nil {
				d.ctx.DeleteMessage(*d.messageId)
			}
			return
		}

		if d.next && time.Now().After(d.t) {
			d.Send()
			d.next = false
		}
		time.Sleep(500 * time.Millisecond)
	}
}
