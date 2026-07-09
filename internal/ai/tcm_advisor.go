package ai

import (
	"context"
	"fmt"
	"strings"

	"chinese-medical/internal/config"
	"chinese-medical/internal/model"
)

type TCMAdvisor struct {
	researcher FoodResearcher
}

type TCMChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type TCMAnswer struct {
	Answer string          `json:"answer"`
	Mode   string          `json:"mode"`
	Intent TCMIntentResult `json:"intent"`
}

func NewTCMAdvisor(cfg config.AIConfig) TCMAdvisor {
	return TCMAdvisor{researcher: NewFoodResearcher(cfg)}
}

func (a TCMAdvisor) Answer(ctx context.Context, question, mode string, history []TCMChatMessage, foods []model.MedicatedFood) (TCMAnswer, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return TCMAnswer{}, fmt.Errorf("question is required")
	}
	intent := AnalyzeTCMIntent(question, mode)

	payload := researchRequest{
		Model:        a.researcher.cfg.ResearchModel,
		Instructions: tcmAdvisorInstructions(intent),
		Input:        tcmAdvisorPrompt(question, intent, history, foods),
	}

	result, err := a.researcher.callModel(ctx, payload)
	if err != nil {
		return TCMAnswer{}, err
	}

	text, _ := researchOutputText(result)
	text = strings.TrimSpace(text)
	if text == "" {
		return TCMAnswer{}, fmt.Errorf("tcm advisor returned empty answer")
	}

	return TCMAnswer{
		Answer: text,
		Mode:   string(intent.Type),
		Intent: intent,
	}, nil
}

func tcmAdvisorInstructions(intent TCMIntentResult) string {
	return fmt.Sprintf(`你是中医智能问答助手，参考中医多智能体问答系统的设计，将问题按模式处理。当前模式是：%s。
识别场景：%s。采用策略：%s。
回答必须使用中文，结构清晰，语气专业克制。
你可以解释中医基础知识、养生调理思路和辨证问诊方向，但不能替代医生诊疗，不能给出确诊、保证疗效、处方剂量或要求用户停止现有治疗。
涉及急症、持续加重、孕产妇、儿童、老人、慢病用药、严重疼痛、胸痛、呼吸困难、意识异常、出血等情况时，必须建议及时就医。`, intent.Type, intent.Label, intent.Strategy)
}

func tcmAdvisorPrompt(question string, intent TCMIntentResult, history []TCMChatMessage, foods []model.MedicatedFood) string {
	var builder strings.Builder
	builder.WriteString("请回答用户的中医相关问题。\n\n")
	builder.WriteString("识别网络结果：\n")
	builder.WriteString("- 场景类型：")
	builder.WriteString(intent.Label)
	builder.WriteString("\n- 核心需求：")
	builder.WriteString(intent.CoreNeed)
	builder.WriteString("\n- 适配策略：")
	builder.WriteString(intent.Strategy)
	builder.WriteString("\n- 情绪识别：")
	builder.WriteString(intent.Emotion)
	if len(intent.MissingInfo) > 0 {
		builder.WriteString("\n- 需要补充的问诊信息：")
		builder.WriteString(strings.Join(intent.MissingInfo, "、"))
	}
	if intent.ExternalCapability != "" {
		builder.WriteString("\n- 外部能力状态：")
		builder.WriteString(intent.ExternalCapability)
	}
	builder.WriteString("\n\n")

	if len(intent.CachedClassics) > 0 {
		builder.WriteString("古籍缓存命中：\n")
		for _, snippet := range intent.CachedClassics {
			builder.WriteString("- ")
			builder.WriteString(snippet)
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}

	if len(foods) > 0 {
		builder.WriteString("本系统知识库中检索到的相关条目，可优先参考，但不要虚构未提供内容：\n")
		for _, item := range foods {
			builder.WriteString(fmt.Sprintf("- #%d %s（%s）\n", item.ID, item.Name, item.Category))
			if strings.TrimSpace(item.Source) != "" {
				builder.WriteString("  来源：")
				builder.WriteString(item.Source)
				builder.WriteString("\n")
			}
			if strings.TrimSpace(item.Food) != "" {
				builder.WriteString("  组成：")
				builder.WriteString(item.Food)
				builder.WriteString("\n")
			}
			if strings.TrimSpace(item.Method) != "" {
				builder.WriteString("  制法：")
				builder.WriteString(item.Method)
				builder.WriteString("\n")
			}
			if strings.TrimSpace(item.Effect) != "" {
				builder.WriteString("  功效：")
				builder.WriteString(item.Effect)
				builder.WriteString("\n")
			}
		}
		builder.WriteString("\n")
	}

	if len(history) > 0 {
		builder.WriteString("最近对话历史：\n")
		start := 0
		if len(history) > 8 {
			start = len(history) - 8
		}
		for _, message := range history[start:] {
			role := "用户"
			if message.Role == "assistant" {
				role = "助手"
			}
			content := strings.TrimSpace(message.Content)
			if content == "" {
				continue
			}
			builder.WriteString(role)
			builder.WriteString("：")
			builder.WriteString(content)
			builder.WriteString("\n")
		}
		builder.WriteString("\n")
	}

	builder.WriteString("用户问题：\n")
	builder.WriteString(question)
	builder.WriteString("\n\n")
	builder.WriteString("输出要求：\n")
	builder.WriteString("1. 先给直接回答。\n")
	builder.WriteString("2. 按识别场景输出对应结构：闲聊类重温和科普；辨证类重追问；古籍类重出处核验；医案类重相似路径；药材类重功效/禁忌/配伍；图文类说明当前需要上传图片能力。\n")
	builder.WriteString("3. 如引用了本系统知识库条目或古籍缓存，请在回答中点名。\n")
	builder.WriteString("4. 如果外部 GraphRAG、Neo4j、TCM-CV 尚未接入，要明确说明当前为降级回答。\n")
	builder.WriteString("5. 结尾用一句话提示：以上内容仅供健康科普参考，不能替代专业医生诊疗。\n")
	return builder.String()
}
