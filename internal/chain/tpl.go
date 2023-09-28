package chain

import (
	"bytes"
	"github.com/bincooo/llm-plugin/internal/repo/store"
	"github.com/sirupsen/logrus"
	"html/template"
	"strings"
	"time"

	autotypes "github.com/bincooo/AutoAI/types"
	"github.com/bincooo/llm-plugin/internal/types"
)

type TplInterceptor struct {
	autotypes.BaseInterceptor
}

func (*TplInterceptor) Before(bot autotypes.Bot, ctx *autotypes.ConversationContext) (bool, error) {
	ctxArgs := ctx.Data.(types.ConversationContextArgs)
	content := strings.ReplaceAll(ctx.Prompt, "\"", "\\u0022")
	kv := map[string]any{
		"content": content,
		"args":    ctxArgs,
		"online":  store.GetOnline(ctx.Id),
		"date":    time.Now().Format("2006-01-02 15:04:05"),
	}

	result, err := tplHandle(ctx.Prompt, kv)
	if err != nil {
		return false, err
	}
	ctx.Prompt = result

	delete(kv, "content")
	result, err = tplHandle(ctx.Preset, kv)
	if err != nil {
		return false, err
	}
	ctx.Preset = result
	return true, nil
}

func tplHandle(tmplVar string, context map[string]any) (string, error) {
	tmpl, err := template.New("context").Parse(tmplVar)
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
