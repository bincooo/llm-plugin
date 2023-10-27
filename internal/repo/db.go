package repo

import (
	"errors"
	"github.com/FloatTech/floatbox/ctxext"
	sql "github.com/FloatTech/sqlite"
	"github.com/bincooo/llm-plugin/internal/vars"
	nano "github.com/fumiama/NanoBot"
	"github.com/sirupsen/logrus"
	"strings"
	"sync"
	"time"
)

type command struct {
	sql *sql.Sqlite
	sync.RWMutex
}

type GlobalConfig struct {
	Id    int    `db:"id"`
	Proxy string `db:"proxy"` // 代理
	Bot   string `db:"bot"`   // AI类型
	Role  string `db:"role"`  // 默认预设
}

type TokenConfig struct {
	Id        string `db:"id"`
	Key       string `db:"key"`
	Type      string `db:"type"`       // 类型
	AppId     string `db:"app_id"`     // Claude APPID
	Token     string `db:"token"`      // 凭证
	BaseURL   string `db:"base_url"`   // 代理转发
	MaxTokens int    `db:"max_tokens"` // openai-api 最大Tokens
	Images    int    `db:"images"`     // 是否支持图片识别
}

type RoleConfig struct {
	Id      string `db:"id"`
	Key     string `db:"key"`
	Type    string `db:"type"`    // 类型
	Content string `db:"content"` // 预设内容
	Message string `db:"message"` // 消息模版
	Chain   string `db:"chain"`   // 拦截处理器
	Section int    `db:"section"` // 是否分段输出
}

var (
	cmd = &command{
		sql: &sql.Sqlite{},
	}

	OnceOnSuccess = ctxext.DoOnceOnSuccess(func(ctx *nano.Ctx) bool {
		ready, err := postRef()
		if err != nil {
			logrus.Error(err)
			_, _ = ctx.SendPlainMessage(false, err.Error())
		}
		return ready
	})
)

func init() {
	// 等待ZeroBot初始化
	go func() {
		for {
			if vars.E != nil {
				_, _ = postRef()
				return
			}
			time.Sleep(time.Second)
		}
	}()
}

func postRef() (bool, error) {
	if vars.E == nil {
		return false, errors.New("ZeroBot未初始化")
	}

	cmd.sql.DBPath = vars.E.DataFolder() + "/storage.db"
	err := cmd.sql.Open(time.Hour * 24)
	if err != nil {
		return false, err
	}

	// 初始化数据表
	err = cmd.sql.Create("global", &GlobalConfig{})
	if err != nil {
		return false, err
	}

	err = cmd.sql.Create("tokens", &TokenConfig{})
	if err != nil {
		return false, err
	}

	err = cmd.sql.Create("roles", &RoleConfig{})
	if err != nil {
		return false, err
	}

	return true, nil
}

func GetGlobal() *GlobalConfig {
	var g GlobalConfig
	if err := cmd.sql.Find("global", &g, ""); err != nil {
		g = GlobalConfig{
			Id:  1,
			Bot: "openai-api",
		}
	}
	return &g
}

func EditGlobal(g GlobalConfig) error {
	cmd.Lock()
	defer cmd.Unlock()
	return cmd.sql.Insert("global", &g)
}

func EditToken(token TokenConfig) error {
	cmd.Lock()
	defer cmd.Unlock()
	return cmd.sql.Insert("tokens", &token)
}

func GetToken(id, key, t string) *TokenConfig {
	var token TokenConfig
	where := make([]string, 0)
	if id != "" {
		where = append(where, " id='"+id+"'")
	}
	if key != "" {
		where = append(where, " key='"+key+"'")
	}
	if t != "" {
		where = append(where, " type='"+t+"'")
	}

	w := ""
	if len(where) > 0 {
		w = "where" + strings.Join(where, "and")
	}

	err := cmd.sql.Find("tokens", &token, w)
	if err != nil {
		return nil
	}
	return &token
}

func FindTokens(key, t string) ([]*TokenConfig, error) {
	where := make([]string, 0)
	if key != "" {
		where = append(where, " key='"+key+"'")
	}
	if t != "" {
		where = append(where, " type='"+t+"'")
	}

	w := ""
	if len(where) > 0 {
		w = "where" + strings.Join(where, "and")
	}

	return sql.FindAll[TokenConfig](cmd.sql, "tokens", w)
}

func RemoveToken(id string) {
	_ = cmd.sql.Del("tokens", "where id='"+id+"'")
}

// 通过ID、key（名称）、t（ai类型）获取角色配置
func GetRole(id, key, t string) *RoleConfig {
	var p RoleConfig
	where := make([]string, 0)
	if id != "" {
		where = append(where, " id='"+id+"'")
	}
	if key != "" {
		where = append(where, " key='"+key+"'")
	}
	if t != "" {
		where = append(where, " type='"+t+"'")
	}

	w := ""
	if len(where) > 0 {
		w = "where" + strings.Join(where, "and")
	}

	err := cmd.sql.Find("roles", &p, w)
	if err != nil {
		logrus.Error(err)
		return nil
	}
	return &p
}

func EditRole(role RoleConfig) error {
	cmd.Lock()
	defer cmd.Unlock()
	return cmd.sql.Insert("roles", &role)
}

func FindRoles(key, t string) ([]*RoleConfig, error) {
	where := make([]string, 0)
	if key != "" {
		where = append(where, " key='"+key+"'")
	}
	if t != "" {
		where = append(where, " type='"+t+"'")
	}

	w := ""
	if len(where) > 0 {
		w = "where" + strings.Join(where, "and")
	}

	return sql.FindAll[RoleConfig](cmd.sql, "roles", w)
}

func RemoveRole(id string) {
	_ = cmd.sql.Del("roles", "where id='"+id+"'")
}
