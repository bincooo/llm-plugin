package llm

import (
	"github.com/bincooo/llm-plugin/internal/repo"
	"strconv"
	"strings"
)

const gTpl = `
Bot = "[BOT]"
Role = "[ROLE]"
Proxy = "[PROXY]"
`

const tokenTpl = `
Key = "[KEY]"
Type = "[TYPE]"
AppId = "[APP_ID]"
Token = "[TOKEN]"
MaxTokens = [MAX_TOKENS]
BaseURL = "[BASE_URL]"
Images = [IMAGES]
`

const roleTpl = `
Key = "[KEY]"
Type = "[TYPE]"
Chain = "[CHAIN]"
Section = [SECTION]

Content = """
[CONTENT]
"""

Message = """"
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
