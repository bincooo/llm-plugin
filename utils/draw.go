package utils

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/bincooo/AutoAI/utils"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"strings"
)

const gTpl = `
{
    "enable_hr": false,
    "hr_scale": 2,
    "hr_upscaler": "4x-UltraSharp",
    "hr_second_pass_steps": 15,
    "hr_resize_x": 0,
    "hr_resize_y": 0,
    "denoising_strength": 0.5,
    "styles": [],
    "seed": -1,
    "subseed": -1,
    "subseed_strength": 15,
    "seed_resize_from_h": 0,
    "seed_resize_from_w": 0,
    "sampler_name": "DPM++ 2M Karras",
    "sampler_index": "DPM++ 2M Karras",
    "batch_size": 1,
    "n_iter": 1,
    "steps": 30,
    "cfg_scale": 10,
    "width": 512,
    "height": 640,
    "restore_faces": false,
    "tiling": false,
    "prompt": "[prompt]",
    "negative_prompt": "nsfw",
    "script_args": [],
    "script_name": null
}`

type DrawBody struct {
	EnableHr          bool           `json:"enable_hr"`
	HrScale           float64        `json:"hr_scale"`
	HrUpscaler        string         `json:"hr_upscaler"`
	HrSecondPassSteps int            `json:"hr_second_pass_steps"`
	HrResizeX         int            `json:"hr_resize_x"`
	HrResizeY         int            `json:"hr_resize_y"`
	DenoisingStrength float64        `json:"denoising_strength"`
	Styles            []any          `json:"styles"`
	Seed              int            `json:"seed"`
	Subseed           int            `json:"subseed"`
	SubseedStrength   int            `json:"subseed_strength"`
	SeedResizeFromH   int            `json:"seed_resize_from_h"`
	SeedResizeFromW   int            `json:"seed_resize_from_w"`
	SamplerName       string         `json:"sampler_name"`
	SamplerIndex      string         `json:"sampler_index"`
	BatchSize         int            `json:"batch_size"`
	NIter             int            `json:"n_iter"`
	Steps             int            `json:"steps"`
	CfgScale          int            `json:"cfg_scale"`
	Width             int            `json:"width"`
	Height            int            `json:"height"`
	RestoreFaces      bool           `json:"restore_faces"`
	Tiling            bool           `json:"tiling"`
	Prompt            string         `json:"prompt"`
	NegativePrompt    string         `json:"negative_prompt"`
	ScriptArgs        []any          `json:"script_args"`
	ScriptName        any            `json:"script_name"`
	OverrideSettings  map[string]any `json:"override_settings"`
	AlwaysOnScripts   map[string]any `json:"alwayson_scripts"`
}

func DrawAI(proxy, bu, prompt, tpl string) ([]byte, error) {
	// 获取实际地址
	if bu != "" && strings.HasPrefix(bu, "redirect:") {
		r, err := utils.NewHttp().GET(bu[9:]).
			AddHeader("Content-Type", "application/json").
			Build()
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		if r.StatusCode != http.StatusOK {
			return nil, errors.New(string(body))
		}

		bu = string(body)
	}

	var body DrawBody
	if err := json.Unmarshal([]byte(gTpl), &body); err != nil {
		return nil, err
	}
	if tpl != "" {
		if err := json.Unmarshal([]byte(tpl), &body); err != nil {
			return nil, err
		}
	}

	prompt = strings.ReplaceAll(prompt, "\"", "")
	prompt = strings.ReplaceAll(prompt, "，", ",")

	if strings.Contains(body.Prompt, "[prompt]") {
		body.Prompt = strings.Replace(body.Prompt, "[prompt]", prompt, -1)
	} else {
		body.Prompt += prompt
	}

	marshal, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	builder := utils.NewHttp().POST(bu+"/sdapi/v1/txt2img", bytes.NewReader(marshal)).
		AddHeader("Content-Type", "application/json")
	if proxy != "" {
		builder.ProxyString(proxy)
	}
	r, err := builder.Build()
	if err != nil {
		return nil, err
	}

	if r.StatusCode != 200 {
		b, _ := io.ReadAll(r.Body)
		return nil, errors.New(r.Status + "::" + string(b))
	}

	marshal, err = io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	var jm map[string]any
	if err = json.Unmarshal(marshal, &jm); err != nil {
		return nil, err
	}

	images, ok := jm["images"]
	if !ok {
		logrus.Warn("作画失败：", marshal)
		return nil, errors.New("生成失败")
	}

	str := images.([]any)[0]
	imgBytes, err := base64.StdEncoding.DecodeString(str.(string))
	if err != nil {
		return nil, err
	}

	return imgBytes, nil
}
