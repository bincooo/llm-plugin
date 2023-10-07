package llm

import (
	"errors"
	"github.com/FloatTech/floatbox/file"
	"github.com/bincooo/llm-plugin/utils"
	"github.com/sirupsen/logrus"
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
	if maker.kv == nil {
		maker.kv = make(map[string]ttsApi)
	}
	maker.kv[k] = api
}

func (maker *TTSMaker) Audio(k, tone, tex string) (string, error) {
	logrus.Info("开始文本转语音: ", k, tone)
	if api, ok := maker.kv[k]; ok {
		return api.Audio(tone, tex)
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
		tex += "** " + k + " **\n" + strings.Join(api.Tones(), "\n") + "\n\n"
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

	path, err := speech.URL(speech.FileName)
	if err != nil {
		return "", err
	}

	return "file:///" + file.BOTPATH + "/" + path, nil
}

// 语音风格列表
func (tts *_edgeTts) Tones() []string {
	return []string{
		"zh-CN-XiaoxiaoNeural",
	}
}
