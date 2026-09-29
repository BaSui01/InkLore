package enginepromptctx

import (
	enginedomain "InkLore/internal/agent/domain"
	"bytes"
	"fmt"
	"text/template"
)

type AuxiliaryModel struct {
	ID       int64  `json:"id"`
	Model    string `json:"model"`
	Provider string `json:"provider"`
}
type AuxiliaryTaskBinding struct {
	Task  enginedomain.AgentAuxiliaryTask
	Model AuxiliaryModel
}

func RenderAuxiliaryPrompt(source, conversation, instruction string) (string, error) {
	if source == "" {
		return "", fmt.Errorf("辅助任务提示词未配置")
	}
	t, err := template.New("agent_auxiliary").Option("missingkey=error").Parse(source)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	err = t.Execute(&out, map[string]string{"Conversation": conversation, "Instruction": instruction})
	return out.String(), err
}
