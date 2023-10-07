package llm

import (
	"errors"
	"github.com/FloatTech/floatbox/file"
	"github.com/bincooo/llm-plugin/utils"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	edgetts "github.com/pp-group/edge-tts-go"
	edgebiz "github.com/pp-group/edge-tts-go/biz/service/tts/edge"
)

var (
	Xieyin = map[string]string{
		"a": "`诶`",
		"b": "`必`",
		"c": "`西`",
		"d": "`地`",
		"e": "`亿`",
		"f": "`哎辅`",
		"g": "`计`",
		"h": "`哎曲`",
		"i": "`爱`",
		"j": "`戒`",
		"k": "`剋`",
		"l": "`哎乐`",
		"m": "`哎母`",
		"o": "`欧`",
		"p": "`批`",
		"q": "`扣`",
		"r": "`啊`",
		"s": "`哎死`",
		"t": "`踢`",
		"u": "`优`",
		"v": "`微`",
		"w": "`答不留`",
		"x": "`爱克斯`",
		"y": "`歪`",
		"z": "`贼`",
	}
)

type TTSMaker struct {
	kv map[string]ttsApi
}

type ttsApi interface {
	// 文本转语音
	Audio(tone, tex string) ([]string, error)
	// 语音风格列表
	Tones() []string
}

func (maker *TTSMaker) Reg(k string, api ttsApi) {
	if maker.kv == nil {
		maker.kv = make(map[string]ttsApi)
	}
	maker.kv[k] = api
}

func (maker *TTSMaker) Audio(k, tone, tex string) ([]string, error) {
	logrus.Info("开始文本转语音: ", k, tone)
	if api, ok := maker.kv[k]; ok {
		return api.Audio(tone, tex)
	} else {
		return nil, errors.New("未定义的语音api类型")
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
func (tts *_edgeTts) Audio(tone, tex string) ([]string, error) {
	communicate, err := edgebiz.NewCommunicate(tex, edgebiz.Option{
		OptID: 1,
		Param: tone,
	})
	if err != nil {
		return nil, err
	}

	speech, err := edgetts.NewLocalSpeech(communicate, "data")
	if err != nil {
		return nil, err
	}

	_, exec := speech.GenTTS()
	if err = exec(); err != nil {
		return nil, err
	}

	path, err := speech.URL(speech.FileName)
	if err != nil {
		return nil, err
	}

	return []string{"file:///" + file.BOTPATH + "/" + path}, nil
}

// 语音风格列表
func (tts *_edgeTts) Tones() []string {
	return []string{
		"zh-CN-XiaoxiaoNeural",
	}
}

// ========================
var genshinvoiceBaseUrl = "https://genshinvoice.top/api"

type _genshinvoice struct {
}

// 文本转语音
func (tts *_genshinvoice) Audio(tone, tex string) ([]string, error) {
	tex = strings.ToLower(tex)
	for k, v := range Xieyin {
		tex = strings.ReplaceAll(tex, k, v)
	}

	max := 200
	slice := make([]string, 0)
	r := []rune(tex)

	count := len(r) / max
	if count == 0 {
		count = 1
	}
	if n := len(r) % max; n > 0 {
		if n > 5 {
			count++
		}
	}

	for i := 0; i < count; i++ {
		l := len(r)
		end := i*max + max
		if end > l {
			end = l
		}
		msg := r[i*max : end]
		if end+5 >= l {
			msg = r[i*max : l]
		}
		response, err := http.Get(genshinvoiceBaseUrl + "?speaker=" + url.QueryEscape(tone) + "&text=" + url.QueryEscape(string(msg)) +
			"&format=wav&noise=0.9&noisew=0.9&sdp_ratio=0.2")
		if err != nil {
			logrus.Error("语音生成失败: ", err)
			return nil, errors.New("生成失败")
		}

		if response.StatusCode != http.StatusOK {
			return nil, errors.New("生成失败: " + response.Status)
		}

		audio, err := io.ReadAll(response.Body)
		if err != nil {
			return nil, errors.New("生成失败")
		}

		if strings.Contains(response.Header.Get("Content-Type"), "text/html") {
			return nil, errors.New(string(audio))
		}
		wav := "data/genshivoice_" + uuid.NewString() + ".wav"
		err = os.WriteFile(wav, audio, 0666)
		if err != nil {
			return nil, err
		}
		slice = append(slice, "file:///"+file.BOTPATH+"/"+wav)
	}
	return slice, nil
}

// 语音风格列表
func (tts *_genshinvoice) Tones() []string {
	return []string{
		"丹恒", "克拉拉", "穹", "「信使」", "史瓦罗", "彦卿", "晴霓", "杰帕德", "素裳",
		"绿芙蓉", "罗刹", "艾丝妲", "黑塔", "丹枢", "希露瓦", "白露", "费斯曼", "停云",
		"可可利亚", "景元", "螺丝咕姆", "青镞", "公输师傅", "卡芙卡", "大毫", "驭空", "半夏",
		"奥列格", "娜塔莎", "桑博", "瓦尔特", "阿兰", "伦纳德", "佩拉", "卡波特", "帕姆", "帕斯卡",
		"青雀", "三月七", "刃", "姬子", "布洛妮娅", "希儿", "星", "符玄", "虎克", "银狼", "镜流",
		"「博士」", "「大肉丸」", "九条裟罗", "佐西摩斯", "刻晴", "博易", "卡维", "可莉", "嘉玛", "埃舍尔",
		"塔杰·拉德卡尼", "大慈树王", "宵宫", "康纳", "影", "枫原万叶", "欧菲妮", "玛乔丽", "珊瑚", "田铁嘴",
		"砂糖", "神里绫华", "罗莎莉亚", "荒泷一斗", "莎拉", "迪希雅", "钟离", "阿圆", "阿娜耶", "阿拉夫", "雷泽",
		"香菱", "龙二", "「公子」", "「白老先生」", "优菈", "凯瑟琳", "哲平", "夏洛蒂", "安柏", "巴达维", "式大将",
		"斯坦利", "毗伽尔", "海妮耶", "爱德琳", "纳西妲", "老孟", "芙宁娜", "阿守", "阿祇", "丹吉尔", "丽莎", "五郎",
		"元太", "克列门特", "克罗索", "北斗", "埃勒曼", "天目十五", "奥兹", "恶龙", "早柚", "杜拉夫", "松浦", "柊千里",
		"甘雨", "石头", "纯水精灵？", "羽生田千鹤", "莱依拉", "菲谢尔", "言笑", "诺艾尔", "赛诺", "辛焱", "迪娜泽黛",
		"那维莱特", "八重神子", "凯亚", "吴船长", "埃德", "天叔", "女士", "恕筠", "提纳里", "派蒙", "流浪者", "深渊使徒",
		"玛格丽特", "珐露珊", "琴", "瑶瑶", "留云借风真君", "绮良良", "舒伯特", "荧", "莫娜", "行秋", "迈勒斯", "阿佩普",
		"鹿野奈奈", "七七", "伊迪娅", "博来", "坎蒂丝", "埃尔欣根", "埃泽", "塞琉斯", "夜兰", "常九爷", "悦", "戴因斯雷布",
		"笼钓瓶一心", "纳比尔", "胡桃", "艾尔海森", "艾莉丝", "菲米尼", "蒂玛乌斯", "迪奥娜", "阿晃", "阿洛瓦",
		"陆行岩本真蕈·元素生命", "雷电将军", "魈", "鹿野院平藏", "「女士」", "「散兵」", "凝光", "妮露", "娜维娅",
		"宛烟", "慧心", "托克", "托马", "掇星攫辰天君", "旁白", "浮游水蕈兽·元素生命", "烟绯", "玛塞勒", "百闻", "知易",
		"米卡", "西拉杰", "迪卢克", "重云", "阿扎尔", "霍夫曼", "上杉", "久利须", "嘉良", "回声海螺", "多莉", "安西",
		"德沃沙克", "拉赫曼", "林尼", "查尔斯", "深渊法师", "温迪", "爱贝尔", "珊瑚宫心海", "班尼特", "琳妮特", "申鹤",
		"神里绫人", "艾伯特", "萍姥姥", "萨赫哈蒂", "萨齐因", "阿尔卡米", "阿贝多", "anzai", "久岐忍", "九条镰治", "云堇",
		"伊利亚斯", "埃洛伊", "塞塔蕾", "拉齐", "昆钧", "柯莱", "沙扎曼", "海芭夏", "白术", "空", "艾文", "芭芭拉", "莫塞伊思",
		"莺儿", "达达利亚", "迈蒙", "长生", "阿巴图伊", "陆景和", "莫弈", "夏彦", "左然",
	}
}
