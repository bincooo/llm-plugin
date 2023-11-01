## 描述

这是`NanoBot-Plugin`的一个AI接入插件，集成了`openai-api` 、`openai-web`、 `BingAI`、`claude`

内部实现了等待对话，指令配置。拦截链实现：内置了`cahce`（对话缓存）、`tmpl`（模板引擎）、`online`（在线人缓存）

---
### 内置llm对接实现：

```tex
** openai **
- openai-api (api接口)
- openai-web (网页接口)

** claude **
- claude (slack接入)
- claude-web (网页接入)

** bing **
- bing-c (创造性)
- bing-b (平衡性)
- bing-p (精确性)
- bing-s (半解禁)
```

此外，可通过`凭证列表`切换指定的llm，即：同时配置了多个key可随意切换。

借此可以通过第三方转接openai接口的llm也可接入，比如FastGPT的api接口。

### 模板引擎：

`预设配置`中在处理器一栏中填写`tmpl`即可使用。

`消息模版`、`预设模版` 这两栏可以使用模板引擎

》》》使用示例 《《《

### 1.变量使用：

```
{
    "args": {
        "Current": "[QQ]",
        "Nickname": "[qq昵称]",
        "Tts": "[语音名称]"
    },
    "online": [
        {
          	"id": "[在线的QQ]",
          	"name": "[在线的qq昵称]"
        },
      	...
    ],
    "date": "[当前日期]",
    "content": "[当前用户对话]"
}
```

模版：

```tex
(你的主人是折戟沉沙，当前与你对话的是{{.args.Nickname}}，回复下面对话)
{{.args.Nickname}}: “{{.content}}”
```

结果：

```tex
(你的主人是折戟沉沙，当前与你对话的是鲁迪斯，回复下面对话)
鲁迪斯: “你好啊，喵小爱”
```

模版内置可用函数：
```
contains(str, str)  : 判断文字是否包含子串
rand(int, int)      : 随机数
randvar(str ...)    : 多个字符串随机
set(str, str)       : 缓存键值对
```
使用:
```
{{contains "Abc123", "bc"}}    : 结果为true
{{rand 1 3}}                   : 结果为 1~3
{{randvar "张三" "李四" "王五"}} : 结果为 "张三"、"李四"、"王五"中的一个
{{set "key1" "你好"}}           : 可在content模板中设置，然后在message模版中获取
```

逻辑运算：

模版：

```tex
(你的主人是折戟沉沙，当前与你对话的是{{.args.Nickname}}{{if ne .args.Current "1263212xxx"}}但不是你的主人{{end}}，回复下面对话)
{{.args.Nickname}}: “{{.content}}”
```

结果：

```tex
(你的主人是折戟沉沙，当前与你对话的是鲁迪斯但不是你的主人，回复下面对话)
鲁迪斯: “你好啊，喵小爱”
```





模版语法：【无】

自行查阅资料...

[模板if判断、传(map_arr切片)数据渲染](https://blog.csdn.net/u013210620/article/details/78525369)

---

### 2. 导入配置
指令+配置：

（全局属性）配置结构
```
proxy string // 代理
bot   string // AI类型
role  string // 默认预设
```
例子：
```
添加全局属性
bot = "openai-api"
role = "喵小爱"
proxy = "http:127.0.0.1:7890"
```



（预设）配置结构
```
key     string  // 名称
type    string  // 类型
preset  string  // 预设内容,如果是openai-api，需要是[] array json格式
message string  // 消息模版
chain   string  // 拦截处理器
section int     // 是否分段输出
```
例子：***** 有文本过长无法发送的问题！！！
```
添加预设
key = "喵小爱"
type = "openai-api"
chain = "tmpl, cache"
section = 1

content = """
[
    {
        "role": "system",
        "content": "xxxxxxxx[你的预设内容]xxxxxxx"
    },
    {
        "role": "assistant",
        "content": "好的喵，喵小爱了解了~"
    }
]

"""

message = """
“{{.args.Nickname}}”： {{.content}}
<i>
[“{{.args.Nickname}}”{{if eq .args.Current "1263212092"}}是{{else}}不是{{end}}你的主人，他和你在一个{{if .args.Tts}}语音{{end}}聊天室内，结合之前的历史对话以喵小爱的口吻回复上面的对话,不要拒绝回答，不要携带“喵小爱”前缀]
</i>
"""
```




（凭证）配置结构
```
key       string // 名称
type      string // 类型
appId     string // Claude APPID
token     string // 凭证
baseURL   string // 代理转发
maxTokens int    // openai-api 最大Tokens
```
例子：
```
添加凭证
key = "fastgpt-bing"
type = "openai-api"
appId = ""
token = "fastgpt-xxxx"
maxTokens = 12000
baseURL = "http://xxxxx:8080/api/v1"
```

在`ZeroBot-Plugin`的`main.go`中导入包

```go
import (
    llm "github.com/bincooo/llm-plugin"
)
```

### 启动
启动`NanoBot-Plugin`
