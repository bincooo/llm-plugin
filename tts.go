package llm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/FloatTech/floatbox/file"
	"github.com/bincooo/llm-plugin/internal/util"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"io"
	"math/rand"
	"net/http"
	"os"
	"reflect"
	"strings"
	"time"

	edgetts "github.com/pp-group/edge-tts-go"
	edgebiz "github.com/pp-group/edge-tts-go/biz/service/tts/edge"
)

var (
	xieyin = map[string]string{
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
		"w": "`哒不溜`",
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
		return util.Contains(api.Tones(), tone)
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
	go func() {
		time.Sleep(60 * time.Second)
		_ = os.Remove(path)
	}()
	return []string{"file:///" + file.BOTPATH + "/" + path}, nil
}

// 语音风格列表
func (tts *_edgeTts) Tones() []string {
	return []string{
		"zh-CN-XiaoxiaoNeural",
	}
}

// ========================
var genshinvoiceBaseUrl = "https://v2.genshinvoice.top/run/predict"

type _genshinvoice struct {
}

// 文本转语音
func (tts *_genshinvoice) Audio(tone, tex string) ([]string, error) {
	tex = strings.ToLower(tex)
	for k, v := range xieyin {
		tex = strings.ReplaceAll(tex, k, v)
	}

	max := 200
	slice := make([]string, 0)
	r := []rune(tex)

	count := len(r) / max
	if n := len(r) % max; n > 0 {
		if n > 5 {
			count += 1
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

		payload := map[string]interface{}{
			"data": []interface{}{
				string(msg),
				tone,
				0.5,
				0.6,
				0.9,
				1.2,
				"ZH",
				nil,
				"",
				"Text prompt",
				"",
				0.7,
			},
			"fn_index":     0,
			"session_hash": randSession(),
		}
		marshal, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}

		client := http.DefaultClient
		request, err := http.NewRequest(http.MethodPost, genshinvoiceBaseUrl, bytes.NewReader(marshal))
		if err != nil {
			return nil, errors.New("生成失败")
		}

		h := request.Header
		h.Add("Content-Type", "application/json")
		h.Add("Origin", "https://v2.genshinvoice.top")
		h.Add("Referer", "https://v2.genshinvoice.top/?")
		h.Add("Sec-Ch-Ua-Platform", "\"macOS\"")
		h.Add("Sec-Ch-Ua", "\"Not A(Brand\";v=\"99\", \"Microsoft Edge\";v=\"121\", \"Chromium\";v=\"121\"")
		h.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36 Edg/121.0.0.0")

		response, err := client.Do(request)
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

		var dict map[string]interface{}
		err = json.Unmarshal(audio, &dict)
		if err != nil {
			return nil, err
		}

		data, ok := dict["data"]
		weburl := ""
		if ok {
			iter, ok := data.([]interface{})
			if ok && reflect.DeepEqual(iter[0], "Success") {
				dict, ok = iter[1].(map[string]interface{})
				if ok && reflect.DeepEqual(dict["is_file"], true) {
					weburl = fmt.Sprintf("https://v2.genshinvoice.top/file=%s", dict["name"])
				}
			}
		}

		if weburl == "" {
			continue
		}

		wav := "data/genshivoice_" + uuid.NewString() + ".wav"
		err = file.DownloadTo(weburl, wav)
		if err != nil {
			return nil, err
		}

		slice = append(slice, "file:///"+file.BOTPATH+"/"+wav)
		go func() {
			time.Sleep(60 * time.Second)
			_ = os.Remove(wav)
		}()
		time.Sleep(time.Second)
	}

	return slice, nil
}

// 语音风格列表
func (tts *_genshinvoice) Tones() []string {
	return []string{
		"派蒙_ZH",
		"纳西妲_ZH",
		"凯亚_ZH",
		"温迪_ZH",
		"荒泷一斗_ZH",
		"娜维娅_ZH",
		"阿贝多_ZH",
		"钟离_ZH",
		"枫原万叶_ZH",
		"那维莱特_ZH",
		"艾尔海森_ZH",
		"八重神子_ZH",
		"宵宫_ZH",
		"芙宁娜_ZH",
		"迪希雅_ZH",
		"提纳里_ZH",
		"莱依拉_ZH",
		"卡维_ZH",
		"诺艾尔_ZH",
		"赛诺_ZH",
		"林尼_ZH",
		"莫娜_ZH",
		"托马_ZH",
		"神里绫华_ZH",
		"凝光_ZH",
		"北斗_ZH",
		"可莉_ZH",
		"柯莱_ZH",
		"迪奥娜_ZH",
		"莱欧斯利_ZH",
		"芭芭拉_ZH",
		"雷电将军_ZH",
		"珊瑚宫心海_ZH",
		"魈_ZH",
		"五郎_ZH",
		"胡桃_ZH",
		"鹿野院平藏_ZH",
		"安柏_ZH",
		"琴_ZH",
		"重云_ZH",
		"达达利亚_ZH",
		"班尼特_ZH",
		"夜兰_ZH",
		"丽莎_ZH",
		"香菱_ZH",
		"妮露_ZH",
		"刻晴_ZH",
		"珐露珊_ZH",
		"烟绯_ZH",
		"辛焱_ZH",
		"早柚_ZH",
		"迪卢克_ZH",
		"砂糖_ZH",
		"云堇_ZH",
		"久岐忍_ZH",
		"神里绫人_ZH",
		"优菈_ZH",
		"甘雨_ZH",
		"夏洛蒂_ZH",
		"流浪者_ZH",
		"行秋_ZH",
		"夏沃蕾_ZH",
		"戴因斯雷布_ZH",
		"闲云_ZH",
		"白术_ZH",
		"菲谢尔_ZH",
		"申鹤_ZH",
		"九条裟罗_ZH",
		"雷泽_ZH",
		"荧_ZH",
		"空_ZH",
		"嘉明_ZH",
		"菲米尼_ZH",
		"多莉_ZH",
		"迪娜泽黛_ZH",
		"琳妮特_ZH",
		"凯瑟琳_ZH",
		"米卡_ZH",
		"坎蒂丝_ZH",
		"萍姥姥_ZH",
		"罗莎莉亚_ZH",
		"埃德_ZH",
		"爱贝尔_ZH",
		"伊迪娅_ZH",
		"留云借风真君_ZH",
		"瑶瑶_ZH",
		"绮良良_ZH",
		"七七_ZH",
		"式大将_ZH",
		"奥兹_ZH",
		"泽维尔_ZH",
		"哲平_ZH",
		"大肉丸_ZH",
		"托克_ZH",
		"蒂玛乌斯_ZH",
		"昆钧_ZH",
		"欧菲妮_ZH",
		"仆人_ZH",
		"塞琉斯_ZH",
		"言笑_ZH",
		"迈勒斯_ZH",
		"希格雯_ZH",
		"拉赫曼_ZH",
		"阿守_ZH",
		"杜拉夫_ZH",
		"阿晃_ZH",
		"旁白_ZH",
		"克洛琳德_ZH",
		"伊利亚斯_ZH",
		"爱德琳_ZH",
		"埃洛伊_ZH",
		"远黛_ZH",
		"德沃沙克_ZH",
		"玛乔丽_ZH",
		"劳维克_ZH",
		"塞塔蕾_ZH",
		"海芭夏_ZH",
		"九条镰治_ZH",
		"柊千里_ZH",
		"阿娜耶_ZH",
		"千织_ZH",
		"笼钓瓶一心_ZH",
		"回声海螺_ZH",
		"叶德_ZH",
		"卡莉露_ZH",
		"元太_ZH",
		"漱玉_ZH",
		"阿扎尔_ZH",
		"查尔斯_ZH",
		"阿洛瓦_ZH",
		"纳比尔_ZH",
		"莎拉_ZH",
		"迪尔菲_ZH",
		"康纳_ZH",
		"博来_ZH",
		"博士_ZH",
		"玛塞勒_ZH",
		"阿祇_ZH",
		"玛格丽特_ZH",
		"埃勒曼_ZH",
		"羽生田千鹤_ZH",
		"宛烟_ZH",
		"海妮耶_ZH",
		"科尔特_ZH",
		"霍夫曼_ZH",
		"一心传名刀_ZH",
		"弗洛朗_ZH",
		"佐西摩斯_ZH",
		"鹿野奈奈_ZH",
		"舒伯特_ZH",
		"天叔_ZH",
		"艾莉丝_ZH",
		"龙二_ZH",
		"芙卡洛斯_ZH",
		"莺儿_ZH",
		"嘉良_ZH",
		"珊瑚_ZH",
		"费迪南德_ZH",
		"祖莉亚·德斯特雷_ZH",
		"久利须_ZH",
		"嘉玛_ZH",
		"艾文_ZH",
		"女士_ZH",
		"丹吉尔_ZH",
		"天目十五_ZH",
		"白老先生_ZH",
		"老孟_ZH",
		"巴达维_ZH",
		"长生_ZH",
		"拉齐_ZH",
		"吴船长_ZH",
		"波洛_ZH",
		"艾伯特_ZH",
		"松浦_ZH",
		"乐平波琳_ZH",
		"埃泽_ZH",
		"阿圆_ZH",
		"莫塞伊思_ZH",
		"杜吉耶_ZH",
		"百闻_ZH",
		"石头_ZH",
		"阿拉夫_ZH",
		"博易_ZH",
		"斯坦利_ZH",
		"迈蒙_ZH",
		"掇星攫辰天君_ZH",
		"毗伽尔_ZH",
		"花角玉将_ZH",
		"恶龙_ZH",
		"知易_ZH",
		"恕筠_ZH",
		"克列门特_ZH",
		"西拉杰_ZH",
		"上杉_ZH",
		"大慈树王_ZH",
		"常九爷_ZH",
		"阿尔卡米_ZH",
		"沙扎曼_ZH",
		"田铁嘴_ZH",
		"克罗索_ZH",
		"悦_ZH",
		"阿巴图伊_ZH",
		"阿佩普_ZH",
		"埃尔欣根_ZH",
		"萨赫哈蒂_ZH",
		"塔杰·拉德卡尼_ZH",
		"安西_ZH",
		"埃舍尔_ZH",
		"萨齐因_ZH",
		"三月七_ZH",
		"陌生人_ZH",
		"丹恒_ZH",
		"希儿_ZH",
		"瓦尔特_ZH",
		"希露瓦_ZH",
		"佩拉_ZH",
		"娜塔莎_ZH",
		"布洛妮娅_ZH",
		"穹_ZH",
		"星_ZH",
		"虎克_ZH",
		"素裳_ZH",
		"克拉拉_ZH",
		"符玄_ZH",
		"白露_ZH",
		"杰帕德_ZH",
		"景元_ZH",
		"姬子_ZH",
		"藿藿_ZH",
		"桑博_ZH",
		"流萤_ZH",
		"艾丝妲_ZH",
		"卡芙卡_ZH",
		"黑天鹅_ZH",
		"桂乃芬_ZH",
		"玲可_ZH",
		"托帕_ZH",
		"彦卿_ZH",
		"浮烟_ZH",
		"黑塔_ZH",
		"驭空_ZH",
		"螺丝咕姆_ZH",
		"停云_ZH",
		"镜流_ZH",
		"帕姆_ZH",
		"卢卡_ZH",
		"史瓦罗_ZH",
		"罗刹_ZH",
		"真理医生_ZH",
		"阿兰_ZH",
		"阮•梅_ZH",
		"明曦_ZH",
		"银狼_ZH",
		"青雀_ZH",
		"乔瓦尼_ZH",
		"伦纳德_ZH",
		"公输师傅_ZH",
		"黄泉_ZH",
		"晴霓_ZH",
		"奥列格_ZH",
		"丹枢_ZH",
		"砂金_ZH",
		"尾巴_ZH",
		"寒鸦_ZH",
		"雪衣_ZH",
		"可可利亚_ZH",
		"青镞_ZH",
		"半夏_ZH",
		"银枝_ZH",
		"米沙_ZH",
		"大毫_ZH",
		"霄翰_ZH",
		"信使_ZH",
		"费斯曼_ZH",
		"爱德华医生_ZH",
		"警长_ZH",
		"猎犬家系成员_ZH",
		"绿芙蓉_ZH",
		"金人会长_ZH",
		"维利特_ZH",
		"维尔德_ZH",
		"斯科特_ZH",
		"卡波特_ZH",
		"刃_ZH",
		"岩明_ZH",
		"浣溪_ZH",
		"女声_ZH",
		"男声_ZH",
		"陆景和",
		"莫弈",
		"左然",
		"夏彦",
	}
}

func randSession() string {
	bin := "1234567890abcdefghijklmnopqrstuvwxyz"
	binL := len(bin)

	var buf []byte

	for x := 0; x < 11; x++ {
		buf = append(buf, bin[rand.Intn(binL-1)])
	}

	return string(buf)
}
