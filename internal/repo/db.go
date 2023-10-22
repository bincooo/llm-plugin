package repo

import (
	"bytes"
	"errors"
	"github.com/FloatTech/floatbox/ctxext"
	sql "github.com/FloatTech/sqlite"
	"github.com/bincooo/llm-plugin/internal/vars"
	"github.com/sirupsen/logrus"
	zero "github.com/wdvxdr1123/ZeroBot"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode"
)

type command struct {
	sql *sql.Sqlite
	sync.RWMutex
}

type GlobalConfig struct {
	Id        int    `db:"id" json:"id"`
	Proxy     string `db:"proxy" json:"proxy"`           // 代理
	NbServ    string `db:"nb_serv" json:"nb_serv"`       // newbing 服务地址
	Bot       string `db:"bot" json:"bot"`               // AI类型
	MaxTokens int    `db:"max_tokens" json:"max_tokens"` // openai-api 最大Tokens
	Preset    string `db:"preset" json:"preset"`         // 默认预设
}

type TokenConfig struct {
	Id      string `db:"id" json:"id"`
	Key     string `db:"key" json:"key" query:"like"`
	Type    string `db:"type" json:"type" query:"="` // 类型
	Email   string `db:"email" json:"email"`         // 邮箱
	Passwd  string `db:"passwd" json:"passwd"`       // 密码
	AppId   string `db:"claude_bot" json:"app_id"`   // Claude APPID
	Token   string `db:"token" json:"token"`         // 凭证
	BaseURL string `db:"base_url" json:"base_url"`   // 代理转发
	Expire  string `db:"expire" json:"expire"`       // 过期日期
}

type RoleConfig struct {
	Id      string `db:"id" json:"id"`
	Key     string `db:"key" json:"key" query:"like"`
	Type    string `db:"type" json:"type" query:"="` // 类型
	Content string `db:"content" json:"content"`     // 预设内容
	Message string `db:"message" json:"message"`     // 消息模版
	Chain   string `db:"chain" json:"chain"`         // 拦截处理器
	Section B2Int  `db:"section" json:"section"`     // 是否分段输出
}

// bool转int
type B2Int int

func (b *B2Int) MarshalJSON() ([]byte, error) {
	if *b == 1 {
		return []byte("true"), nil
	} else {
		return []byte("false"), nil
	}
}

func (b *B2Int) UnmarshalJSON(bt []byte) (err error) {
	if len(bt) < 4 {
		return
	}
	if bytes.Equal(bt, []byte("true")) {
		*b = 1
	}
	if bytes.Equal(bt, []byte("false")) {
		*b = 0
	}
	return
}

var (
	cmd = &command{
		sql: &sql.Sqlite{},
	}

	OnceOnSuccess = ctxext.DoOnceOnSuccess(func(ctx *zero.Ctx) bool {
		ready, err := postRef()
		if err != nil {
			ctx.Send(err.Error())
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

	err = cmd.sql.Create("token", &TokenConfig{})
	if err != nil {
		return false, err
	}

	err = cmd.sql.Create("preset_scene", &RoleConfig{})
	if err != nil {
		return false, err
	}

	return true, nil
}

// 构建查询条件
func BuildCondition(model any) string {
	var condition = ""
	v := reflect.ValueOf(model)
	for index := 0; index < v.NumField(); index++ {
		db, ok1 := v.Type().Field(index).Tag.Lookup("db")
		query, ok2 := v.Type().Field(index).Tag.Lookup("query")
		if !ok1 || !ok2 {
			continue
		}
		s := v.Field(index).String()
		if s == "" {
			continue
		}
		if query == "like" {
			condition += db + " " + query + " '%" + s + "%' and "
		} else {
			condition += db + " " + query + " '" + s + "' and "
		}
	}

	if condition != "" {
		cut, ok := strings.CutSuffix(condition, " and ")
		if ok {
			condition = "where " + cut
		} else {
			condition = "where " + condition
		}
	}
	return condition
}

func (c *command) Count(table string, condition string) (num int, err error) {
	if c.sql.DB == nil {
		return 0, sql.ErrNilDB
	}
	stmt, err := cmd.sql.DB.Prepare("SELECT COUNT(1) FROM " + wraptable(table) + condition + ";")
	if err != nil {
		return 0, err
	}
	rows, err := stmt.Query()
	if err != nil {
		return 0, err
	}
	if rows.Err() != nil {
		return 0, rows.Err()
	}
	if rows.Next() {
		err = rows.Scan(&num)
	}
	err = rows.Close()
	if err != nil {
		return 0, err
	}
	return num, err
}

func wraptable(table string) string {
	first := []rune(table)[0]
	if first < unicode.MaxLatin1 && unicode.IsDigit(first) {
		return "[" + table + "]"
	} else {
		return "'" + table + "'"
	}
}

func GetGlobal() GlobalConfig {
	var g GlobalConfig
	if err := cmd.sql.Find("global", &g, ""); err != nil {
		g = GlobalConfig{
			Id:  1,
			Bot: "openai-api",
		}
	}
	return g
}

func InsertGlobal(g GlobalConfig) error {
	cmd.Lock()
	defer cmd.Unlock()
	return cmd.sql.Insert("global", &g)
}

func SetProxy(p string) error {
	cmd.Lock()
	defer cmd.Unlock()
	global := GetGlobal()
	global.Proxy = p
	return cmd.sql.Insert("global", &global)
}

func InsertToken(token TokenConfig) error {
	cmd.Lock()
	defer cmd.Unlock()
	var t TokenConfig
	err := cmd.sql.Find("token", &t, "where type='"+token.Type+"' and key='"+token.Key+"'")
	if err != nil {
		return cmd.sql.Insert("token", &token)
	} else {
		return errors.New("`" + token.Key + "`已存在")
	}
}

func UpdateToken(t TokenConfig) {
	cmd.Lock()
	defer cmd.Unlock()
	if err := cmd.sql.Insert("token", &t); err != nil {
		logrus.Warn(err)
	}
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

	err := cmd.sql.Find("token", &token, w)
	if err != nil {
		return nil
	}
	return &token
}

func FindTokens(t string) ([]*TokenConfig, error) {
	if t != "" {
		return sql.FindAll[TokenConfig](cmd.sql, "token", "where type='"+t+"'")
	} else {
		return sql.FindAll[TokenConfig](cmd.sql, "token", "")
	}
}

func RemoveToken(key string) {
	cmd.sql.Del("token", "where key='"+key+"'")
}

func GetPresetScene(id, key, t string) *RoleConfig {
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

	err := cmd.sql.Find("preset_scene", &p, w)
	if err != nil {
		return nil
	}
	return &p
}

func FindPresetScenes(t string) ([]*RoleConfig, error) {
	if t != "" {
		return sql.FindAll[RoleConfig](cmd.sql, "preset_scene", "where type='"+t+"'")
	} else {
		return sql.FindAll[RoleConfig](cmd.sql, "preset_scene", "")
	}
}
