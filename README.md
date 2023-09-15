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
下载miaox.zip数据资源文件解压到`ZeroBot-Plugin`的`data`目录下
```text
|- ZeroBot-Plugin
    |- data
        |- miaox
```