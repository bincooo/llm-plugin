package util

import (
	"errors"
	"github.com/bincooo/edge-api/util"
	"github.com/bincooo/llm-plugin/internal/repo/store"
	nano "github.com/fumiama/NanoBot"
	"github.com/sirupsen/logrus"
	"regexp"
	"strconv"
	"strings"
)

// String转换消息对象MessageSegment
func StringToMessageSegment(uid, msg string) []nano.MessageSegment {
	compileRegex := regexp.MustCompile(`\[@[^]]+]`)
	matches := compileRegex.FindAllStringSubmatch(msg, -1)
	logrus.Info("StringToMessageSegment CQ:At:: ", matches)
	pos := 0
	online := store.GetOnline(uid)
	var slice []nano.MessageSegment
	if len(online) == 0 {
		goto label
	}
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

		contain := ContainFor(online, func(item store.OKeyv) bool {
			if item.Name == qq {
				qq = item.Id
			}
			return item.Id == qq
		})

		if contain {
			slice = append(slice, nano.Text(msg[pos:index]))
			pos = index + len(qq) + 3
			slice = append(slice, nano.At(strings.TrimSpace(qq)))
		}
	}
label:
	if len(msg)-1 > pos {
		slice = append(slice, nano.Text(msg[pos:]))
	}

	return slice
}

// 尝试解决Bing人机检验
func HandleBingCaptcha(token string, err error) {
	content := err.Error()
	if strings.Contains(content, "User needs to solve CAPTCHA to continue") {
		// content = "用户需要人机验证...  已尝试自动验证，若重新生成文本无效请手动验证。"
		if strings.Contains(token, "_U=") {
			split := strings.Split(token, ";")
			for _, item := range split {
				if strings.Contains(item, "_U=") {
					token = strings.TrimSpace(strings.ReplaceAll(item, "_U=", ""))
					break
				}
			}
		}
		if e := util.SolveCaptcha(token); e != nil {
			logrus.Error("尝试解析Bing人机检验失败：", e)
		}
	}
}

func Retry(count int, exec func() error) error {
	if count <= 0 {
		return errors.New("请提供有效的重试次数")
	}

	for i := 0; i <= count; i++ {
		if err := exec(); err != nil {
			if i == count {
				return err
			}
			logrus.Warn("重试中["+strconv.Itoa(i+1)+"]: ", err)
			continue
		} else {
			break
		}
	}
	return nil
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
