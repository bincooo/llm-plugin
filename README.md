## 描述

这是`ZeroBot-Plugin`的一个AI接入插件，集成了`openai-api` 、`openai-web`、 `BingAI`、`claude`

内部实现了等待对话，web可视化配置。拦截链实现：内置了`cahce`（对话缓存）、`tmpl`（模板引擎）、`online`（在线人缓存）

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

`web`页`预设配置`，在处理器一栏中填写`tmpl`即可使用。

`消息模版`、`预设模版` 这两栏可以使用模板引擎

》》》使用示例 《《《

变量使用：

```json
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

step 1:
在`ZeroBot-Plugin`的`main.go`中导入包

```go
import (
    llm "github.com/bincooo/llm-plugin"
)
```

setp 2:
在`ZeroBot-Plugin`的`main.go`大概在263行处添加`llm.Run(":8081")`

```go
// 启用 webui
// go webctrl.RunGui(*g)
llm.Run(":8081")
if *runcfg != "" {
```

setp 3:
下载`data-1.zip`数据资源文件：

[点我下载](https://github.com/bincooo/llm-plugin/archive/refs/tags/data-1.zip) 解压并将目录名改名`miaox`移动至`ZeroBot-Plugin`的`data`目录下
```text
|- ZeroBot-Plugin
    |- data
        |- miaox
```

或者下载源码，复制data文件夹到`ZeroBot-Plugin/data/data` 并改名为 `ZeroBot-Plugin/data/miaox`

### -
启动`ZeroBot-Plugin`后在浏览器内访问 http://127.0.0.1:8081 配置接入信息即可
