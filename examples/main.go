package main

import (
	"github.com/bincooo/chatgpt-adapter"
	"github.com/bincooo/chatgpt-adapter/types"
	"github.com/bincooo/chatgpt-adapter/vars"
)

func main() {
	manager := adapter.NewBotManager()
	context := Context()
	context.Prompt = "嗨"
	manager.Reply(context, func(response types.PartialResponse) {

	})
	//	beginTime := time.Now()
	//	_, err := utils.DrawAI("", "redirect:http://114.132.201.94:8082/sdapi/v1/redirect", "1girl", `{
	//    "alwayson_scripts": {
	//    },
	//    "enable_hr": true,
	//    "hr_scale": 2,
	//    "hr_upscaler": "R-ESRGAN 4x+",
	//    "denoising_strength": 0.3,
	//    "batch_size": 1,
	//    "cfg_scale": 7,
	//    "height": 616,
	//    "negative_prompt": "(low quality, worst quality), EasyNegativeV2,",
	//    "override_settings": {
	//        "sd_model_checkpoint": "absolutereality_v181.safetensors",
	//        "sd_vae": "Automatic"
	//    },
	//    "clip_skip": 2,
	//    "prompt": "1girl",
	//    "restore_faces": false,
	//    "sampler_index": "DPM++ 2M SDE Karras",
	//    "sampler_name": "",
	//    "script_args": [],
	//    "seed": -1,
	//    "subseed": 0,
	//    "steps": 20,
	//    "tiling": false,
	//    "width": 496
	//}`)
	//	if err != nil {
	//		fmt.Println(err)
	//	}
	//	seconds := time.Now().Sub(beginTime).Seconds()
	//	fmt.Println("耗时：" + strconv.FormatFloat(seconds, 'f', 0, 64) + "s")
}

func Context() types.ConversationContext {
	return types.ConversationContext{
		Id:  "1008611",
		Bot: vars.OpenAIAPI,
		//Bot:     vars.OpenAIWeb,
		Token:   "",
		Preset:  "",
		Format:  "1234",
		Chain:   "replace,cache,assist",
		BaseURL: "https://ai.fakeopen.com/api",
		//Model:   edge.Sydney,
	}
}
