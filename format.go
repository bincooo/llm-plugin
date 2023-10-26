package llm

import (
	"github.com/bincooo/llm-plugin/internal/repo"
	"strconv"
	"strings"
)

const gTpl = `
bot = "[BOT]"
role = "[ROLE]"
proxy = "[PROXY]"
`

const tokenTpl = `
key = "[KEY]"
type = "[TYPE]"
appId = "[APP_ID]"
token = "[TOKEN]"
maxTokens = [MAX_TOKENS]
baseURL = "[BASE_URL]"
images = [IMAGES]
`

const roleTpl = `
key = "[KEY]"
type = "[TYPE]"
chain = "[CHAIN]"
section = [SECTION]

content = """
[CONTENT]
"""

message = """"
[MESSAGE]
"""
`

func formatGlobal(g *repo.GlobalConfig) string {
	if g == nil {
		g = &repo.GlobalConfig{}
	}
	content := strings.Replace(gTpl, "[BOT]", g.Bot, -1)
	content = strings.Replace(content, "[ROLE]", g.Role, -1)
	content = strings.Replace(content, "[PROXY]", g.Proxy, -1)
	return content
}

func formatToken(token *repo.TokenConfig) string {
	if token == nil {
		token = &repo.TokenConfig{}
	}
	content := strings.Replace(tokenTpl, "[KEY]", token.Key, -1)
	content = strings.Replace(content, "[TYPE]", token.Type, -1)
	content = strings.Replace(content, "[APP_ID]", token.AppId, -1)
	content = strings.Replace(content, "[TOKEN]", token.Token, -1)
	content = strings.Replace(content, "[MAX_TOKENS]", strconv.Itoa(token.MaxTokens), -1)
	content = strings.Replace(content, "[IMAGES]", strconv.Itoa(token.Images), -1)
	content = strings.Replace(content, "[BASE_URL]", token.BaseURL, -1)
	return content
}

func formatRole(role *repo.RoleConfig) string {
	if role == nil {
		role = &repo.RoleConfig{}
	}
	content := strings.Replace(roleTpl, "[KEY]", role.Key, -1)
	content = strings.Replace(content, "[TYPE]", role.Type, -1)
	content = strings.Replace(content, "[CHAIN]", role.Chain, -1)
	content = strings.Replace(content, "[SECTION]", strconv.Itoa(role.Section), -1)
	content = strings.Replace(content, "[MESSAGE]", role.Message, -1)
	content = strings.Replace(content, "[CONTENT]", role.Content, -1)
	return content
}
