package ai

import (
	"strings"
)

type TCMIntentType string

const (
	TCMIntentAuto          TCMIntentType = "auto"
	TCMIntentChat          TCMIntentType = "tcm-chat"
	TCMIntentDiagnose      TCMIntentType = "tcm-diagnose"
	TCMIntentClassics      TCMIntentType = "classics-search"
	TCMIntentCaseReference TCMIntentType = "case-reference"
	TCMIntentHerb          TCMIntentType = "herb-consult"
	TCMIntentImageAnalysis TCMIntentType = "image-analysis"
)

type TCMIntentResult struct {
	Type               TCMIntentType `json:"type"`
	Label              string        `json:"label"`
	CoreNeed           string        `json:"core_need"`
	Strategy           string        `json:"strategy"`
	Confidence         float64       `json:"confidence"`
	Emotion            string        `json:"emotion"`
	MissingInfo        []string      `json:"missing_info"`
	MatchedSignals     []string      `json:"matched_signals"`
	CachedClassics     []string      `json:"cached_classics,omitempty"`
	ExternalCapability string        `json:"external_capability,omitempty"`
}

type tcmIntentProfile struct {
	label              string
	coreNeed           string
	strategy           string
	externalCapability string
}

var tcmIntentProfiles = map[TCMIntentType]tcmIntentProfile{
	TCMIntentChat: {
		label:    "中医闲聊类",
		coreNeed: "中医基础知识、日常养生咨询和概念解释",
		strategy: "动态情感分析：识别用户焦虑或健康担忧，调整为温和科普语气",
	},
	TCMIntentDiagnose: {
		label:    "辨证问诊类",
		coreNeed: "寒热虚实辨证、症状线索梳理和追问信息补全",
		strategy: "多轮追问策略：按寒热、汗出、二便、饮食睡眠、舌苔、脉象补全辨证关键信息",
	},
	TCMIntentClassics: {
		label:    "古籍检索类",
		coreNeed: "中医经典条文、方义和古籍出处查询",
		strategy: "缓存优先机制：优先返回内置高频古籍片段，再提示进一步核验",
	},
	TCMIntentCaseReference: {
		label:              "医案参考类",
		coreNeed:           "相似病例、症状-辨证-治法路径参考",
		strategy:           "语义匹配检索：关联症状、辨证和治法，返回相似医案参考",
		externalCapability: "GraphRAG 医案库尚未接入，当前先基于本地调理方知识库和模型知识降级回答",
	},
	TCMIntentHerb: {
		label:              "药材咨询类",
		coreNeed:           "药材功效、禁忌、配伍和长期使用风险咨询",
		strategy:           "知识关联响应：返回功效、禁忌、配伍注意和就医边界",
		externalCapability: "Neo4j 药材关系图尚未接入，当前先基于模型和本地知识库降级回答",
	},
	TCMIntentImageAnalysis: {
		label:              "图文解析类",
		coreNeed:           "舌苔、面色等图文特征辅助分析",
		strategy:           "多模态集成：识别舌苔颜色、厚薄、润燥等特征后辅助辨证",
		externalCapability: "TCM-CV 多模态模型尚未接入，当前文本接口不能直接分析图片",
	},
}

func AnalyzeTCMIntent(question, requestedMode string) TCMIntentResult {
	question = strings.TrimSpace(question)
	intentType := normalizeTCMIntentType(requestedMode)
	signals := []string{}
	if intentType == TCMIntentAuto {
		intentType, signals = classifyTCMIntent(question)
	} else {
		signals = append(signals, "用户手动选择模式")
	}

	profile := tcmIntentProfiles[intentType]
	result := TCMIntentResult{
		Type:               intentType,
		Label:              profile.label,
		CoreNeed:           profile.coreNeed,
		Strategy:           profile.strategy,
		Confidence:         confidenceForSignals(signals),
		Emotion:            detectTCMEmotion(question),
		MissingInfo:        missingDiagnoseInfo(question, intentType),
		MatchedSignals:     signals,
		CachedClassics:     cachedClassics(question, intentType),
		ExternalCapability: profile.externalCapability,
	}
	return result
}

func normalizeTCMIntentType(mode string) TCMIntentType {
	switch TCMIntentType(strings.TrimSpace(mode)) {
	case TCMIntentChat, TCMIntentDiagnose, TCMIntentClassics, TCMIntentCaseReference, TCMIntentHerb, TCMIntentImageAnalysis:
		return TCMIntentType(strings.TrimSpace(mode))
	default:
		return TCMIntentAuto
	}
}

func classifyTCMIntent(question string) (TCMIntentType, []string) {
	type score struct {
		intent  TCMIntentType
		points  int
		signals []string
	}
	scores := []score{
		{intent: TCMIntentImageAnalysis},
		{intent: TCMIntentClassics},
		{intent: TCMIntentCaseReference},
		{intent: TCMIntentHerb},
		{intent: TCMIntentDiagnose},
		{intent: TCMIntentChat},
	}

	add := func(intent TCMIntentType, points int, signal string) {
		for i := range scores {
			if scores[i].intent == intent {
				scores[i].points += points
				scores[i].signals = append(scores[i].signals, signal)
				return
			}
		}
	}

	matchAny := func(words ...string) bool {
		for _, word := range words {
			if strings.Contains(question, word) {
				return true
			}
		}
		return false
	}

	if matchAny("舌苔", "舌象", "舌质", "舌头", "面色", "照片", "图片", "拍照") {
		add(TCMIntentImageAnalysis, 4, "出现舌象/面色/图片相关词")
	}
	if matchAny("伤寒论", "金匮", "黄帝内经", "温病条辨", "条文", "原文", "古籍", "经典", "桂枝汤", "麻黄汤") {
		add(TCMIntentClassics, 4, "出现古籍/经典条文相关词")
	}
	if matchAny("医案", "病例", "案例", "相似", "临床", "诊疗案例") {
		add(TCMIntentCaseReference, 4, "出现医案/病例参考相关词")
	}
	if matchAny("药材", "中药", "黄芪", "当归", "党参", "茯苓", "甘草", "禁忌", "配伍", "长期吃", "能否长期") {
		add(TCMIntentHerb, 4, "出现药材/禁忌/配伍相关词")
	}
	if matchAny("怕冷", "怕热", "流清涕", "黄涕", "咳嗽", "头痛", "发热", "失眠", "便秘", "腹泻", "是什么证", "辨证", "虚实", "寒热") {
		add(TCMIntentDiagnose, 3, "出现症状或辨证相关词")
	}
	if matchAny("养生", "调理", "春季", "夏季", "秋季", "冬季", "饮食", "食补", "体质", "如何养") {
		add(TCMIntentChat, 2, "出现养生咨询相关词")
	}

	best := scores[len(scores)-1]
	for _, item := range scores {
		if item.points > best.points {
			best = item
		}
	}
	if best.points == 0 {
		best.intent = TCMIntentChat
		best.signals = []string{"未命中特定场景，按中医基础问答处理"}
	}
	return best.intent, best.signals
}

func confidenceForSignals(signals []string) float64 {
	switch {
	case len(signals) >= 2:
		return 0.9
	case len(signals) == 1:
		if strings.Contains(signals[0], "未命中") {
			return 0.62
		}
		return 0.82
	default:
		return 0.6
	}
}

func detectTCMEmotion(question string) string {
	if containsAny(question, "焦虑", "害怕", "担心", "严重吗", "怎么办", "急", "会不会", "危险") {
		return "健康担忧/焦虑"
	}
	if containsAny(question, "谢谢", "请问", "想了解") {
		return "平稳求知"
	}
	return "中性"
}

func missingDiagnoseInfo(question string, intentType TCMIntentType) []string {
	if intentType != TCMIntentDiagnose && intentType != TCMIntentImageAnalysis {
		return nil
	}
	checks := []struct {
		name  string
		words []string
	}{
		{name: "寒热表现", words: []string{"怕冷", "怕热", "发热", "恶寒", "寒", "热"}},
		{name: "汗出情况", words: []string{"汗", "出汗", "无汗", "盗汗", "自汗"}},
		{name: "二便情况", words: []string{"大便", "小便", "便秘", "腹泻", "尿"}},
		{name: "饮食睡眠", words: []string{"食欲", "胃口", "睡", "失眠", "多梦"}},
		{name: "舌苔舌质", words: []string{"舌", "舌苔", "舌质"}},
		{name: "脉象信息", words: []string{"脉", "脉象", "浮", "沉", "弦", "滑"}},
	}
	missing := []string{}
	for _, check := range checks {
		if !containsAny(question, check.words...) {
			missing = append(missing, check.name)
		}
	}
	return missing
}

func cachedClassics(question string, intentType TCMIntentType) []string {
	if intentType != TCMIntentClassics {
		return nil
	}
	snippets := []string{}
	if containsAny(question, "桂枝汤", "桂枝") {
		snippets = append(snippets, "《伤寒论》桂枝汤相关：常用于太阳中风、发热汗出、恶风、脉缓等证候语境；需结合原文核验。")
	}
	if containsAny(question, "麻黄汤", "麻黄") {
		snippets = append(snippets, "《伤寒论》麻黄汤相关：常用于太阳伤寒、恶寒发热、无汗而喘等证候语境；需结合原文核验。")
	}
	if containsAny(question, "伤寒论") && len(snippets) == 0 {
		snippets = append(snippets, "已命中《伤寒论》高频检索场景；当前仅内置少量常用方证缓存，完整古籍检索库待接入。")
	}
	return snippets
}

func containsAny(text string, words ...string) bool {
	for _, word := range words {
		if strings.Contains(text, word) {
			return true
		}
	}
	return false
}
