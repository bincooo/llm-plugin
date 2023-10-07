package llm

import (
	"errors"
	"github.com/bincooo/llm-plugin/utils"
	"strings"

	edgetts "github.com/pp-group/edge-tts-go"
	edgebiz "github.com/pp-group/edge-tts-go/biz/service/tts/edge"
)

type TTSMaker struct {
	kv map[string]ttsApi
}

type ttsApi interface {
	// 文本转语音
	Audio(tone, tex string) (string, error)
	// 语音风格列表
	Tones() []string
}

func (maker *TTSMaker) Reg(k string, api ttsApi) {
	maker.kv[k] = api
}

func (maker *TTSMaker) Audio(k, tone, tex string) (string, error) {
	if api, ok := maker.kv[k]; ok {
		return api.Audio(tex, tone)
	} else {
		return "", errors.New("未定义的语音api类型")
	}
}

func (maker *TTSMaker) ContainTone(k, tone string) bool {
	if api, ok := maker.kv[k]; ok {
		return utils.Contains(api.Tones(), tone)
	} else {
		return false
	}
}

func (maker *TTSMaker) Echo() (tex string) {
	for k, api := range maker.kv {
		tex += "** " + k + " **\n" + strings.Join(api.Tones(), "\n")
	}
	if tex == "" {
		tex = "none"
	}
	return
}

// ===========================

type _edgeTts struct {
}

// 文本转语音
func (tts *_edgeTts) Audio(tone, tex string) (string, error) {
	communicate, err := edgebiz.NewCommunicate(tex, edgebiz.Option{
		OptID: 1,
		Param: tone,
	})
	if err != nil {
		return "", err
	}

	speech, err := edgetts.NewLocalSpeech(communicate, "data")
	if err != nil {
		return "", err
	}

	_, exec := speech.GenTTS()
	if err = exec(); err != nil {
		return "", err
	}

	return speech.URL(speech.FileName)
}

// 语音风格列表
func (tts *_edgeTts) Tones() []string {
	return []string{
		"xxx",
	}
}
