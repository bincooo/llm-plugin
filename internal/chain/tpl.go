package chain

import (
	"bytes"
	"github.com/bincooo/llm-plugin/internal/repo/store"
	"github.com/sirupsen/logrus"
	"html/template"
	"math/rand"
	"strings"
	"time"

	autotypes "github.com/bincooo/chatgpt-adapter/types"
	"github.com/bincooo/llm-plugin/internal/types"
)

type TplInterceptor struct {
	autotypes.BaseInterceptor
}

func (*TplInterceptor) Before(bot autotypes.Bot, ctx *autotypes.ConversationContext) (bool, error) {
	ctxArgs := ctx.Data.(types.ConversationContextArgs)
	content := strings.ReplaceAll(ctx.Prompt, "\"", "\\u0022")
	kv := map[string]any{
		"bot":     ctx.Bot,
		"content": content,
		"args":    ctxArgs,
		"online":  store.GetOnline(ctx.Id),
		"date":    time.Now().Format("2006-01-02 15:04:05"),
	}

	if ctx.Preset != "" {
		// delete(kv, "content")
		result, err := tplHandle(ctx.Preset, kv)
		if err != nil {
			return false, err
		}
		ctx.Preset = result
	}

	if ctx.Format != "" {
		result, err := tplHandle(ctx.Format, kv)
		if err != nil {
			return false, err
		}
		ctx.Prompt = result
	}
	return true, nil
}

func tplHandle(tmplVar string, context map[string]any) (string, error) {
	t := template.New("context")
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	// 自定义函数
	funcMap := template.FuncMap{
		"contains": func(s1, s2 any) bool {
			logrus.Info("执行自定义模板函数contains：", s1, s2)
			if _, ok := s1.(string); !ok {
				return false
			}
			if _, ok := s2.(string); !ok {
				return false
			}
			return strings.Contains(s1.(string), s2.(string))
		},
		"rand": func(n1, n2 int) int {
			logrus.Info("执行自定义模板函数rand：", n1, n2)
			return r.Intn(n2-n1) + n1
		},
		"randvar": func(slice ...any) any {
			logrus.Info("执行自定义模板函数randvar：", slice)
			l := len(slice)
			if l == 0 {
				return ""
			}
			return slice[r.Intn(l)]
		},
		"set": func(key string, value any) string {
			logrus.Info("执行自定义模板函数set：", key, value)
			context[key] = value
			return ""
		},
	}
	t.Funcs(funcMap)
	tmpl, err := t.Parse(tmplVar)
	if err != nil {
		logrus.Error("模版引擎构建失败：", err)
		return "", err
	}

	var buffer bytes.Buffer
	if err = tmpl.Execute(&buffer, context); err != nil {
		logrus.Error("模版引擎执行失败：", err)
		return "", err
	}
	return buffer.String(), nil
}
