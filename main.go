package llm

import (
	"encoding/json"
	"github.com/BurntSushi/toml"
	"github.com/FloatTech/floatbox/file"
	"github.com/FloatTech/floatbox/web"
	"github.com/FloatTech/zbputils/control"
	"github.com/FloatTech/zbputils/ctxext"
	"github.com/bincooo/go-openai"
	"github.com/bincooo/llm-plugin/internal/chain"
	"github.com/bincooo/llm-plugin/internal/repo"
	"github.com/bincooo/llm-plugin/internal/repo/store"
	"github.com/bincooo/llm-plugin/internal/types"
	"github.com/bincooo/llm-plugin/internal/util"
	"github.com/bincooo/llm-plugin/internal/vars"
	"github.com/sirupsen/logrus"
	"github.com/wdvxdr1123/ZeroBot/message"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	ctrl "github.com/FloatTech/zbpctrl"
	adapter "github.com/bincooo/chatgpt-adapter"
	adstore "github.com/bincooo/chatgpt-adapter/store"
	adtypes "github.com/bincooo/chatgpt-adapter/types"
	advars "github.com/bincooo/chatgpt-adapter/vars"
	claudevars "github.com/bincooo/claude-api/vars"
	zero "github.com/wdvxdr1123/ZeroBot"
)

const help = `- @Bot + 文本内容
- 昵称前缀 + 文本内容
- 预设列表
- [开启|切换]预设 + [预设名]
- 删除凭证 + [key]
- 添加凭证 + [key]:[value]
- 语音列表
- [开启|切换]语音 [type] [name]
- 关闭语音
- 切换AI + [AI类型：openai-api、openai-web、claude、、、]
- AI列表
`

var (
	engine = control.Register("miaox", &ctrl.Options[*zero.Ctx]{
		Help:              help,
		Brief:             "喵小爱-AI适配器",
		DisableOnDefault:  false,
		PrivateDataFolder: "miaox",
	}).ApplySingle(ctxext.DefaultSingle)

	tts TTSMaker

	lmt adtypes.Limiter

	BB = []string{
		"太啰嗦了巴嘎 ♪(´ε｀ )",
		"这么长的文字你让我咋读啊 (*≧ω≦)",
		"太长了!! 受不了了~ (˶‾᷄ ⁻̫ ‾᷅˵)",
		"你是想把今天的话一次性说完嘛 ( ；´Д｀)",
		"简洁一点点吧，求求了 _(:_」∠)_",
		"要不你自己读读看拟写了什么 (╯‵□′)╯︵┻━┻",
	}
)

// inc:0 自增 priority 0~9, set:0 设置priority
func customPriority(matcher *control.Matcher, priority string) *control.Matcher {
	switch priority[:4] {
	case "set:":
		i, err := strconv.Atoi(priority[4:])
		if err != nil {
			panic(err)
		}
		return (*control.Matcher)((*zero.Matcher)(matcher).SetPriority(i))
	case "inc:":
		i, err := strconv.Atoi(priority[4:])
		if err != nil {
			panic(err)
		}
		if i < 0 || i >= 10 {
			panic("优先级增量范围0～9，实际为: " + strconv.Itoa(i))
		}
		return (*control.Matcher)((*zero.Matcher)(matcher).SetPriority(matcher.Priority + i))
	default:
		panic("未知的优先级指令")
	}
}

func init() {
	vars.E = engine

	var err error
	if !file.IsExist(vars.E.DataFolder() + "/load.gif") {
		data, e := web.GetData("https://cdn.jsdelivr.net/gh/bincooo/llm-plugin@data-1/load.gif")
		if e != nil {
			panic(e)
		}
		_ = os.WriteFile(vars.E.DataFolder()+"/load.gif", data, 0666)
	}
	if vars.Loading, err = os.ReadFile(vars.E.DataFolder() + "/load.gif"); err != nil {
		panic(err)
	}

	// init tts
	tts.Reg("Edge", &_edgeTts{})
	tts.Reg("genshin", &_genshinvoice{})

	lmt = adapter.NewCommonLimiter()
	// init chain
	if e := lmt.RegChain("tmpl", &chain.TplInterceptor{}); e != nil {
		panic(e)
	}
	if e := lmt.RegChain("online", &chain.OnlineInterceptor{}); e != nil {
		panic(e)
	}

	zero.OnNotice(func(ctx *zero.Ctx) bool {
		return ctx.Event.NoticeType == "group_recall" || ctx.Event.NoticeType == "friend_recall"
	}, repo.OnceOnSuccess).SetBlock(false).Handle(recallMessageCommand)

	engine.OnFullMatch("全局属性", zero.AdminPermission, zero.OnlyPrivate, repo.OnceOnSuccess).SetBlock(true).
		Handle(globalCommand)
	engine.OnRegex(`[添加|修改]全局属性\s+([\s\S]*)$`, zero.AdminPermission, zero.OnlyPrivate, repo.OnceOnSuccess).SetBlock(true).
		Handle(editGlobalCommand)
	engine.OnRegex(`[添加|修改]凭证\s+([\s\S]*)$`, zero.AdminPermission, zero.OnlyPrivate, repo.OnceOnSuccess).SetBlock(true).
		Handle(insertTokenCommand)
	engine.OnRegex(`删除凭证\s+(\S+)`, zero.AdminPermission, zero.OnlyPrivate, repo.OnceOnSuccess).SetBlock(true).
		Handle(deleteTokenCommand)
	engine.OnFullMatch("凭证列表", zero.AdminPermission, repo.OnceOnSuccess).SetBlock(true).
		Handle(tokensCommand)
	engine.OnRegex(`凭证明细\s+(\S+)`, zero.AdminPermission, zero.OnlyPrivate, repo.OnceOnSuccess).SetBlock(true).
		Handle(tokenItemCommand)
	engine.OnRegex(`切换凭证\s+(\S+)`, zero.AdminPermission, repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(switchTokensCommand)
	engine.OnFullMatch("AI列表", repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(aiCommand)
	engine.OnRegex(`切换AI\s+(\S+)`, zero.AdminPermission, repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(switchAICommand)
	engine.OnRegex(`[添加|修改]预设\s+([\s\S]*)$`, zero.AdminPermission, zero.OnlyPrivate, repo.OnceOnSuccess).SetBlock(true).
		Handle(insertRoleCommand)
	engine.OnRegex(`删除预设\s+(\S+)`, zero.AdminPermission, zero.OnlyPrivate, repo.OnceOnSuccess).SetBlock(true).
		Handle(deleteRoleCommand)
	engine.OnFullMatch("预设列表", zero.AdminPermission, repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(rolesCommand)
	engine.OnRegex(`预设明细\s+(\S+)`, zero.AdminPermission, zero.OnlyPrivate, repo.OnceOnSuccess).SetBlock(true).
		Handle(roleItemCommand)
	engine.OnRegex(`[开启|切换]预设\s(\S+)`, zero.AdminPermission, repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(switchRoleCommand)
	engine.OnFullMatch("历史对话", zero.AdminPermission, repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(historyCommand)
	engine.OnFullMatch("语音列表", repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(ttsCommand)
	engine.OnFullMatch("关闭语音", repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(closeTTSCommand)
	engine.OnRegex(`[开启|切换]语音\s(.+)`, repo.OnceOnSuccess).SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(switchTTSCommand)
	customPriority(engine.OnMessage(zero.OnlyToMe, repo.OnceOnSuccess), "inc:9").SetBlock(true).Limit(ctxext.LimitByUser).
		Handle(conversationCommand)
}

func globalCommand(ctx *zero.Ctx) {
	g := repo.GetGlobal()

	msg := make(message.Message, 1)
	msg[0] = ctxext.FakeSenderForwardNode(ctx, message.Text(formatGlobal(g)))
	ctx.Send(msg)
}

// 修改全局属性
func editGlobalCommand(ctx *zero.Ctx) {
	value := ctx.State["regex_matched"].([]string)[1]
	value = strings.ReplaceAll(value, "&#91;", "[")
	value = strings.ReplaceAll(value, "&#93;", "]")

	var g repo.GlobalConfig
	if _, err := toml.Decode(value, &g); err != nil {
		logrus.Error(err)
		ctx.Send("添加失败，请按格式填写：" + err.Error())
		return
	}

	if !validateBot(g.Bot) {
		ctx.Send("修改失败，AI类型不正确。可使用：[AI列表] 命令查看")
		return
	}

	dbg := repo.GetGlobal()
	if dbg != nil {
		// 等待用户下一步选择
		if index := waitCommand(ctx, time.Second*60, "已存在相同的全局属性，是否覆盖？", []string{"否", "是"}); index < 1 {
			if index == 0 {
				ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("已取消"))
			}
			return
		}
		g.Id = dbg.Id
	}

	if err := repo.EditGlobal(g); err != nil {
		ctx.Send("修改失败: " + err.Error())
	} else {
		ctx.Send("修改成功")
	}
}

func aiCommand(ctx *zero.Ctx) {
	slice := map[string][]string{
		"openai": {
			"- openai-api (api接口)",
			"- openai-web (网页接口)",
		},
		"claude": {
			"- claude (slack接入)",
			"- claude-web (网页接入)",
		},
		"bing": {
			"- bing-c (创造性)",
			"- bing-b (平衡性)",
			"- bing-p (精确性)",
			"- bing-s (半解禁)",
		},
	}
	tex := ""
	for k, v := range slice {
		tex += "** " + k + " **\n" + strings.Join(v, "\n") + "\n\n"
	}
	ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text(tex))
}

func historyCommand(ctx *zero.Ctx) {
	key := getId(ctx)
	logrus.Info(adstore.GetMessages(key))
	ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("已在后台打印"))
}

// 语音列表
func ttsCommand(ctx *zero.Ctx) {
	ctx.SendChain(message.Text(tts.Echo()))
}

// 关闭语音
func closeTTSCommand(ctx *zero.Ctx) {
	cctx, err := createConversationContext(ctx, "")
	if err != nil {
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("发生异常: "+err.Error()))
		return
	}
	args := cctx.Data.(types.ConversationContextArgs)
	args.Tts = ""
	cctx.Data = args
	updateConversationContext(cctx)
	ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("关闭完毕"))
}

// 开启语音
func switchTTSCommand(ctx *zero.Ctx) {
	matched := ctx.State["regex_matched"].([]string)[1]
	index := strings.Index(matched, " ")
	if index <= 0 || len(matched)-1 == index {
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("参数不正确: "+matched))
		return
	}

	key := strings.TrimSpace(matched[:index])
	value := strings.TrimSpace(matched[index:])
	if !tts.ContainTone(key, value) {
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("不支持的语音类型: "+key+"/"+value))
		return
	}

	cctx, err := createConversationContext(ctx, "")
	if err != nil {
		ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("发生异常: "+err.Error()))
		return
	}
	args := cctx.Data.(types.ConversationContextArgs)
	args.Tts = key + "/" + value
	cctx.Data = args
	updateConversationContext(cctx)
	ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("开启完毕"))
}

// 撤回消息时删除缓存中的消息记录
func recallMessageCommand(ctx *zero.Ctx) {
	// 懒得写switch
	reply := message.Reply(ctx.Event.MessageID)
	messageId := reply.Data["id"]
	adstore.DeleteMessageFor(getId(ctx), messageId)
}

// 聊天
func conversationCommand(ctx *zero.Ctx) {
	name := ctx.Event.Sender.NickName
	if strings.Contains(name, "Q群管家") {
		return
	}

	reply := message.Reply(ctx.Event.MessageID)
	cctx, err := createConversationContext(ctx, "")
	if err != nil {
		ctx.SendChain(reply, message.Text(err))
		logrus.Error(err)
		return
	}

	args := cctx.Data.(types.ConversationContextArgs)
	args.Current = strconv.FormatInt(ctx.Event.Sender.ID, 10)
	args.Nickname = ctx.Event.Sender.NickName
	cctx.Name = args.Nickname
	cctx.Data = args

	images := false
	if dbToken := repo.GetToken(args.Tid, "", ""); dbToken != nil {
		images = dbToken.Images == 1
	}

	prompt := parseMessage(ctx, images)
	messageId := reply.Data["id"]

	// 限制对话长度
	if len([]rune(prompt)) > 300 {
		ctx.SendChain(reply, message.Text(BB[rand.Intn(len(BB))]))
		return
	}

	cctx.Prompt = prompt
	cctx.MessageId = messageId

	section := false
	if role := repo.GetRole(args.Rid, "", ""); role != nil {
		section = role.Section == 1
	}

	timer := util.NewGifTimer(ctx, section)
	cacheMessage := make([]string, 0)
	lmtHandle := func(response adtypes.PartialResponse) {
		if response.Status == advars.Begin {
			timer.Refill()
		}

		if len(strings.TrimSpace(response.Message)) > 0 {
			if section && args.Tts == "" {
				segment := util.StringToMessageSegment(cctx.Id, response.Message)
				ctx.SendChain(append(segment, reply)...)
				timer.Refill()
			} else {
				// 开启语音就不要用分段响应了
				cacheMessage = append(cacheMessage, response.Message)
			}
		}

		if response.Error != nil {
			logrus.Error(response.Error)
			go util.HandleBingCaptcha(cctx.Token, response.Error)
			ctx.SendChain(reply, message.Text(response.Error))
			timer.Release()
			return
		}

		if response.Status == advars.Closed {
			// 开启了语音
			if args.Tts != "" {
				slice := strings.Split(args.Tts, "/")
				msg := strings.TrimSpace(strings.Join(cacheMessage, ""))
				if msg != "" {
					segment := util.StringToMessageSegment(cctx.Id, msg)
					audios, e := tts.Audio(slice[0], slice[1], msg)
					ctx.SendChain(append(segment, reply)...)
					if e != nil {
						ctx.SendChain(reply, message.Text(e))
						logrus.Error("生成语音失败：", e)
					} else {
						for _, audio := range audios {
							time.Sleep(600 * time.Millisecond)
							ctx.SendChain(message.Record(audio))
						}
					}
				}
			} else if !section { // 关闭了分段输出
				msg := strings.TrimSpace(strings.Join(cacheMessage, ""))
				if msg != "" {
					segment := util.StringToMessageSegment(cctx.Id, msg)
					ctx.SendChain(append(segment, message.Reply(ctx.Event.MessageID))...)
				}
			}
			logrus.Info("[MiaoX] - 结束应答")
			timer.Release()
		}
	}

	if e := lmt.Join(cctx, lmtHandle); e != nil {
		ctx.SendChain(reply, message.Text(e))
	}
}

// 添加凭证
func insertTokenCommand(ctx *zero.Ctx) {
	value := ctx.State["regex_matched"].([]string)[1]
	value = strings.ReplaceAll(value, "&#91;", "[")
	value = strings.ReplaceAll(value, "&#93;", "]")

	var newToken repo.TokenConfig
	if _, err := toml.Decode(value, &newToken); err != nil {
		logrus.Error(err)
		ctx.Send("添加失败，请按格式填写：" + err.Error())
		return
	}

	if !validateBot(newToken.Type) {
		ctx.Send("修改失败，AI类型不正确。可使用：[AI列表] 命令查看")
		return
	}

	dbToken := repo.GetToken("", newToken.Key, newToken.Type)
	if dbToken != nil {
		// 等待用户下一步选择
		time.Sleep(3 * time.Second)
		if index := waitCommand(ctx, time.Second*60, "已存在相同的凭证，是否覆盖？", []string{"否", "是"}); index < 1 {
			if index == 0 {
				ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("已取消"))
			}
			return
		}
		newToken.Id = dbToken.Id
	}

	if err := repo.EditToken(newToken); err != nil {
		ctx.Send("添加失败: " + err.Error())
	} else {
		ctx.Send("添加成功")
	}
}

// 删除凭证
func deleteTokenCommand(ctx *zero.Ctx) {
	key := ctx.State["regex_matched"].([]string)[1]
	tokens, err := repo.FindTokens(key, "")
	if err != nil {
		ctx.Send("删除失败: " + err.Error())
		return
	}
	if len(tokens) == 0 {
		ctx.Send("`" + key + "`不存在")
		return
	}

	index := 0
	if len(tokens) > 1 {
		var cmd []string
		for _, dbToken := range tokens {
			cmd = append(cmd, dbToken.Key+"#"+dbToken.Type)
		}
		index = waitCommand(ctx, time.Second*60, "请选择你要删除的凭证", cmd)
		if index == -1 {
			return
		}
	}

	repo.RemoveToken(tokens[index].Id)
	ctx.Send("`" + key + "#" + tokens[0].Type + "`已删除")
}

// 切换AI凭证
func switchTokensCommand(ctx *zero.Ctx) {
	key := ctx.State["regex_matched"].([]string)[1]
	cctx, err := createConversationContext(ctx, "")
	if err != nil {
		ctx.Send("获取上下文出错: " + err.Error())
		return
	}

	bot := cctx.Bot
	if cctx.Bot == advars.Claude && cctx.Model == claudevars.Model4WebClaude2 {
		bot = advars.Claude + "-web"
	}

	token := repo.GetToken("", key, bot)
	if token == nil {
		ctx.Send("`" + cctx.Bot + "`的`" + key + "`凭证不存在")
		return
	}

	if bot != token.Type {
		ctx.Send("当前AI(" + bot + ")无法使用`" + key + "`凭证")
		return
	}

	args := cctx.Data.(types.ConversationContextArgs)
	cctx.Data = args
	cctx.Token = token.Token
	if token.BaseURL != "" {
		cctx.BaseURL = token.BaseURL
	}

	lmt.Remove(cctx.Id, bot)
	store.DeleteOnline(cctx.Id)
	updateConversationContext(cctx)
	ctx.Send("已切换`" + key + "`凭证")
}

// 凭证列表
func tokensCommand(ctx *zero.Ctx) {
	doc := "凭证列表：(bot|key)\n\n"
	tokens, err := repo.FindTokens("", "")
	if err != nil {
		ctx.Send(doc + "None.")
		return
	}
	if len(tokens) <= 0 {
		ctx.Send(doc + "None.")
		return
	}
	for _, token := range tokens {
		doc += padding(token.Type, 20) + " | " + token.Key + "\n"
	}
	ctx.Send(doc)
}

// 凭证明细
func tokenItemCommand(ctx *zero.Ctx) {
	key := ctx.State["regex_matched"].([]string)[1]
	tokens, err := repo.FindTokens(key, "")
	if err != nil {
		ctx.Send("查询失败: " + err.Error())
		return
	}
	if len(tokens) == 0 {
		ctx.Send("`" + key + "`不存在")
		return
	}

	index := 0
	if len(tokens) > 1 {
		var cmd []string
		for _, dbToken := range tokens {
			cmd = append(cmd, dbToken.Key+"#"+dbToken.Type)
		}
		index = waitCommand(ctx, time.Second*60, "请选择你要查看的凭证", cmd)
		if index == -1 {
			return
		}
	}
	msg := make(message.Message, 1)
	msg[0] = ctxext.FakeSenderForwardNode(ctx, message.Text(formatToken(tokens[index])))
	ctx.Send(msg)
}

// 开启/切换预设
func switchRoleCommand(ctx *zero.Ctx) {
	value := ctx.State["regex_matched"].([]string)[1]

	cctx, err := createConversationContext(ctx, "")
	if err != nil {
		ctx.Send("获取上下文出错: " + err.Error())
		return
	}

	// ai类型
	t := cctx.Bot
	if cctx.Bot == advars.Claude && cctx.Model == claudevars.Model4WebClaude2 {
		t = advars.Claude + "-web"
	}

	role := repo.GetRole("", value, t)
	if role == nil {
		ctx.Send("`" + value + "`预设不存在")
		return
	}

	if t != role.Type {
		ctx.Send("当前AI类型无法使用`" + value + "`预设")
		return
	}

	args := cctx.Data.(types.ConversationContextArgs)
	args.Rid = role.Id
	cctx.Data = args

	cctx.Preset = role.Preset
	cctx.Format = role.Message
	cctx.Chain = BaseChain + role.Chain
	bot := cctx.Bot

	lmt.Remove(cctx.Id, bot)
	store.DeleteOnline(cctx.Id)
	updateConversationContext(cctx)
	ctx.Send("已切换`" + value + "`预设")
}

// 添加预设
func insertRoleCommand(ctx *zero.Ctx) {
	value := ctx.State["regex_matched"].([]string)[1]
	value = strings.ReplaceAll(value, "&#91;", "[")
	value = strings.ReplaceAll(value, "&#93;", "]")
	value = strings.ReplaceAll(value, `\n`, "[!n]")
	value = strings.ReplaceAll(value, `\"`, "[!d]")

	var newRole repo.RoleConfig
	if _, err := toml.Decode(value, &newRole); err != nil {
		logrus.Error(err)
		ctx.Send("添加失败，请按格式填写：" + err.Error())
		return
	}

	if newRole.Preset != "" {
		newRole.Preset = strings.ReplaceAll(newRole.Preset, "[!n]", `\n`)
		newRole.Preset = strings.ReplaceAll(newRole.Preset, "[!d]", `\"`)
	}

	if !validateType(newRole.Type) {
		ctx.Send("修改失败，AI类型不正确。请选择以下类型：\n - " +
			advars.OpenAIAPI + "\n - " +
			advars.OpenAIWeb + "\n - " +
			advars.Claude + "\n - " +
			advars.Claude + "-web\n - " +
			advars.Bing)
		return
	}

	if newRole.Type == advars.OpenAIAPI && newRole.Preset != "" {
		var preset []openai.ChatCompletionMessage
		if err := json.Unmarshal([]byte(newRole.Preset), &preset); err != nil {
			logrus.Error("预设解析失败: ", err)
			ctx.Send("预设解析失败: " + err.Error())
			return
		}
	}

	dbRole := repo.GetRole("", newRole.Key, newRole.Type)
	if dbRole != nil {
		// 等待用户下一步选择
		if index := waitCommand(ctx, time.Second*60, "已存在相同的预设，是否覆盖？", []string{"否", "是"}); index < 1 {
			if index == 0 {
				ctx.SendChain(message.Reply(ctx.Event.MessageID), message.Text("已取消"))
			}
			return
		}
		newRole.Id = dbRole.Id
	}

	if err := repo.EditRole(newRole); err != nil {
		ctx.Send("添加失败: " + err.Error())
	} else {
		ctx.Send("添加成功")
	}
}

// 删除预设
func deleteRoleCommand(ctx *zero.Ctx) {
	key := ctx.State["regex_matched"].([]string)[1]
	roles, err := repo.FindRoles(key, "")
	if err != nil {
		ctx.Send("删除失败: " + err.Error())
		return
	}
	if len(roles) == 0 {
		ctx.Send("`" + key + "`不存在")
		return
	}

	index := 0
	if len(roles) > 1 {
		var cmd []string
		for _, dbRole := range roles {
			cmd = append(cmd, dbRole.Key+"#"+dbRole.Type)
		}
		index = waitCommand(ctx, time.Second*60, "请选择你要删除的凭证", cmd)
		if index == -1 {
			return
		}
	}

	repo.RemoveRole(roles[index].Id)
	ctx.Send("`" + key + "#" + roles[0].Type + "`已删除")
}

// 预设列表
func rolesCommand(ctx *zero.Ctx) {
	doc := "预设列表：\n"
	preset, err := repo.FindRoles("", "")
	if err != nil {
		ctx.Send(doc + "None.")
		return
	}
	if len(preset) <= 0 {
		ctx.Send(doc + "None.")
		return
	}
	for _, token := range preset {
		doc += padding(token.Type, 20) + " | " + token.Key + "\n"
	}
	ctx.Send(doc)
}

// 预设明细
func roleItemCommand(ctx *zero.Ctx) {
	key := ctx.State["regex_matched"].([]string)[1]
	roles, err := repo.FindRoles(key, "")
	if err != nil {
		ctx.Send("查询失败: " + err.Error())
		return
	}
	if len(roles) == 0 {
		ctx.Send("`" + key + "`不存在")
		return
	}

	index := 0
	if len(roles) > 1 {
		var cmd []string
		for _, dbRole := range roles {
			cmd = append(cmd, dbRole.Key+"#"+dbRole.Type)
		}
		index = waitCommand(ctx, time.Second*60, "请选择你要查看的预设", cmd)
		if index == -1 {
			return
		}
	}

	msg := make(message.Message, 1)
	msg[0] = ctxext.FakeSenderForwardNode(ctx, message.Text(formatRole(roles[index])))
	ctx.Send(msg)
}

func switchAICommand(ctx *zero.Ctx) {
	bot := ctx.State["regex_matched"].([]string)[1]
	var cctx adtypes.ConversationContext
	switch bot {
	case advars.OpenAIAPI,
		advars.OpenAIWeb,
		advars.Claude,
		advars.Claude + "-web",
		advars.Bing + "-c",
		advars.Bing + "-b",
		advars.Bing + "-p",
		advars.Bing + "-s":
		deleteConversationContext(ctx)
		c, err := createConversationContext(ctx, bot)
		if err != nil {
			ctx.Send(err.Error())
			return
		}
		cctx = c
	default:
		ctx.Send("未知的AI类型：`" + bot + "`")
		return
	}

	lmt.Remove(cctx.Id, cctx.Bot)
	store.DeleteOnline(cctx.Id)
	ctx.Send("已切换`" + bot + "`AI模型")
}

// 等待下一步指令，返回指令下标，-1取消，index对应cmd
func waitCommand(ctx *zero.Ctx, timeout time.Duration, tips string, cmd []string) int {
	cmdtips := ""
	for i, c := range cmd {
		cmdtips += "[" + strconv.Itoa(i) + "] : " + c + "\n"
	}
	ctx.Send(message.Text(tips + "\n发送序号:\n" + cmdtips + " \n发送\"取消\"终止执行"))
	recv, cancel := zero.NewFutureEvent("message", 999, true, zero.RegexRule(`^(取消|\d+)$`), zero.CheckUser(ctx.Event.UserID)).Repeat()
	defer cancel()
	for {
		select {
		case <-time.After(timeout):
			ctx.Send(message.Text("等待超时，已取消"))
			return -1
		case r := <-recv:
			nextcmd := r.Event.Message.String()
			if nextcmd == "取消" {
				ctx.Send(message.Text("已取消"))
				return -1
			}
			index, err := strconv.Atoi(nextcmd)
			if err != nil || index < 0 || index > len(cmd)-1 {
				ctx.Send(message.Text("请输入正确的序号"))
				continue
			}
			return index
		}
	}
}

func padding(value string, num int) string {
	l := len(value) * 2
	if l < num {
		for i := 0; i < num-l; i++ {
			value += " "
		}
	}
	return value
}
