package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"chinese-medical/internal/config"
	"chinese-medical/internal/database"
)

const articleURL = "https://mp.weixin.qq.com/s/xd-tXdWE4SRoV2XlXuLsfw"

type skillSeed struct {
	Name        string
	Slug        string
	Category    string
	Description string
	URL         string
	Tags        string
	Instruction string
}

func main() {
	if err := run(); err != nil {
		slog.Error("import skillhub failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := database.NewPostgresPool(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer db.Close()

	if err := database.EnsureSkillHubSchema(ctx, db); err != nil {
		return fmt.Errorf("ensure skillhub schema: %w", err)
	}

	imported := 0
	for _, item := range articleSkills() {
		if _, err := db.Exec(ctx, `
			INSERT INTO ai_skills (name, slug, category, description, instruction, tags, enabled)
			VALUES ($1, $2, $3, $4, $5, $6, TRUE)
			ON CONFLICT (slug) DO UPDATE SET
				name = EXCLUDED.name,
				category = EXCLUDED.category,
				description = EXCLUDED.description,
				instruction = EXCLUDED.instruction,
				tags = EXCLUDED.tags,
				enabled = TRUE
		`, item.Name, item.Slug, item.Category, item.Description, instructionFor(item), item.Tags); err != nil {
			return fmt.Errorf("upsert skill %s: %w", item.Slug, err)
		}
		imported++
	}

	var total int64
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM ai_skills`).Scan(&total); err != nil {
		return fmt.Errorf("count skillhub rows: %w", err)
	}

	fmt.Printf("imported SkillHub resources from article: %d\n", imported)
	fmt.Printf("skillhub total rows: %d\n", total)
	return nil
}

func instructionFor(item skillSeed) string {
	return fmt.Sprintf(`来源文章：%s
资源地址：%s

使用建议：
%s

落地前请核验项目维护状态、许可证、数据来源、隐私权限和医疗合规边界；涉及真实患者、可穿戴、病历、用药或基因数据时，需要权限控制、审计记录、人工复核和安全边界。`, articleURL, item.URL, item.Instruction)
}

func articleSkills() []skillSeed {
	return []skillSeed{
		{
			Name:        "OpenClaw Medical Skills",
			Slug:        "openclaw-medical-skills",
			Category:    "医疗 Skill 库",
			Description: "869 个医疗 Agent Skill，覆盖文献、病历、药物、基因、生信、临床试验和 FHIR 等方向。",
			URL:         "https://github.com/FreedomIntelligence/OpenClaw-Medical-Skills",
			Tags:        "医疗Agent,Skill,病历,药物,基因,FHIR",
			Instruction: "可作为医疗 Agent 能力地图使用。按具体任务选择子 Skill，并逐个核验 SKILL.md、数据来源和适用边界。",
		},
		{
			Name:        "Apple Health MCP Server",
			Slug:        "apple-health-mcp-server",
			Category:    "可穿戴 MCP",
			Description: "用于读取 Apple Health 数据的 MCP 服务，适合睡眠、步数、心率、体重和运动趋势分析。",
			URL:         "https://github.com/the-momentum/apple-health-mcp-server",
			Tags:        "MCP,Apple Health,可穿戴,健康管理",
			Instruction: "用于接入用户授权的 Apple Health 数据，生成趋势分析、提醒、运动计划或健康管理建议。",
		},
		{
			Name:        "Open Wearables",
			Slug:        "open-wearables",
			Category:    "可穿戴 MCP",
			Description: "自托管可穿戴数据平台，尝试统一 Apple Health、Garmin、Whoop、三星等多来源数据。",
			URL:         "https://github.com/the-momentum/open-wearables",
			Tags:        "MCP,可穿戴,Apple Health,Garmin,健康数据",
			Instruction: "用于把多来源可穿戴数据整理成 AI 可读接口，适合健康管理、体重管理和慢病随访场景。",
		},
		{
			Name:        "Fitbit MCP",
			Slug:        "fitbit-mcp",
			Category:    "可穿戴 MCP",
			Description: "非官方 Fitbit MCP 集成，可作为可穿戴数据接入方向参考。",
			URL:         "https://github.com/NitayRabi/fitbit-mcp",
			Tags:        "MCP,Fitbit,可穿戴,健康数据",
			Instruction: "用于参考 Fitbit 数据接入方式。国内落地时需注意用户覆盖、授权链路和平台可用性。",
		},
		{
			Name:        "Withings MCP",
			Slug:        "withings-mcp",
			Category:    "可穿戴 MCP",
			Description: "非官方 Withings MCP 集成，可作为体重、体脂、血压等设备数据接入方向参考。",
			URL:         "https://github.com/akutishevsky/withings-mcp",
			Tags:        "MCP,Withings,可穿戴,体重管理",
			Instruction: "用于参考 Withings 设备数据接入。国内场景需评估平台可用性、授权稳定性和数据合规。",
		},
		{
			Name:        "OpenFoodFacts MCP",
			Slug:        "openfoodfacts-mcp",
			Category:    "营养运动 MCP",
			Description: "接入 OpenFoodFacts 开源食品库，可查营养标签、过敏原、添加剂和替代选择。",
			URL:         "https://github.com/JagjeevanAK/OpenFoodFacts-MCP",
			Tags:        "MCP,营养,食品库,过敏原,体重管理",
			Instruction: "适合健康零售、营养分析和体重管理。中文商品与国内食品覆盖有限，落地时需要补充本土数据。",
		},
		{
			Name:        "Food & Nutrition MCP",
			Slug:        "food-nutrition-mcp",
			Category:    "营养运动 MCP",
			Description: "饮食计划和营养分析方向 MCP，文章提示项目已停更，仅作方向参考。",
			URL:         "https://github.com/AlwaysSany/food-nutrition",
			Tags:        "MCP,营养,饮食计划,停更参考",
			Instruction: "仅作为饮食计划、营养分析类 Agent 的功能参考。若用于生产，需要重新评估维护状态并替换数据源。",
		},
		{
			Name:        "ExerciseAPI MCP Server",
			Slug:        "exerciseapi-mcp-server",
			Category:    "营养运动 MCP",
			Description: "接入 ExerciseAPI 的 MCP 服务，覆盖 2198 个整理过的动作、肌群、要点和安全提示。",
			URL:         "https://github.com/westvegh/exerciseapi-mcp-server",
			Tags:        "MCP,运动,健身,动作库,安全提示",
			Instruction: "用于避免 AI 凭空编动作，适合健身建议。医学康复、慢病运动处方需补充适应症、禁忌和专业审核。",
		},
		{
			Name:        "Fitness Coach MCP",
			Slug:        "fitness-coach-mcp",
			Category:    "营养运动 MCP",
			Description: "AI 健身追踪应用与 MCP 的结合项目，可参考运动计划和训练追踪工作流。",
			URL:         "https://github.com/Dinesh-Satram/fitness_coach_MCP",
			Tags:        "MCP,健身教练,运动计划,训练追踪",
			Instruction: "适合参考健身计划生成、训练跟踪和用户反馈闭环。医疗级运动处方需另加专业边界。",
		},
		{
			Name:        "LangCare FHIR MCP",
			Slug:        "langcare-fhir-mcp",
			Category:    "医疗系统 MCP",
			Description: "Go 编写的 FHIR MCP，可连接 EPIC、Cerner 及 FHIR R4，内置 40+ 临床 Skill。",
			URL:         "https://github.com/langcare/langcare-mcp-fhir",
			Tags:        "MCP,FHIR,EHR,临床Skill,医疗系统",
			Instruction: "适合连接 FHIR R4 医疗系统，参考用药管理、化验解读、临床决策支持和文书工作流。",
		},
		{
			Name:        "BioMCP",
			Slug:        "biomcp",
			Category:    "生物医学 MCP",
			Description: "统一查询文献、临床试验、变异和药物等多个生物医学库的 MCP。",
			URL:         "https://github.com/genomoncology/biomcp",
			Tags:        "MCP,生物医学,文献,临床试验,变异,药物",
			Instruction: "用于让 Agent 以统一语法查询生物医学数据库，比泛网页搜索更可控，适合科研和药物相关任务。",
		},
		{
			Name:        "BioContextAI Registry",
			Slug:        "biocontextai-registry",
			Category:    "MCP 目录",
			Description: "生物医学 MCP server 目录，收录 PDBe、BioMCP、Cellosaurus 等资源。",
			URL:         "https://biocontext.ai/registry",
			Tags:        "MCP目录,生物医学,Registry",
			Instruction: "用于调研和发现生物医学方向 MCP server。选型时需要逐个核验维护状态、许可证和数据源。",
		},
		{
			Name:        "Awesome Healthcare MCP Servers",
			Slug:        "awesome-healthcare-mcp-servers",
			Category:    "MCP 目录",
			Description: "医疗健康 MCP 合集，项目方标注 HIPAA 等级和临床有效性评分。",
			URL:         "https://github.com/rdmgator12/awesome-healthcare-mcp-servers",
			Tags:        "MCP目录,医疗健康,HIPAA,临床有效性",
			Instruction: "用于早期调研医疗 MCP。评分属于项目方自评，不等同于认证，不能直接当生产安全依据。",
		},
		{
			Name:        "Awesome Healthcare MCP",
			Slug:        "blockrunai-awesome-healthcare-mcp",
			Category:    "MCP 目录",
			Description: "BlockRunAI 维护的医疗健康 MCP 合集，可作为 MCP 资源检索入口。",
			URL:         "https://github.com/BlockRunAI/awesome-healthcare-mcp",
			Tags:        "MCP目录,医疗健康,资源合集",
			Instruction: "用于快速查找医疗健康 MCP 项目。落地前需核验每个子项目的维护状态、数据范围和合规边界。",
		},
		{
			Name:        "AIPOCH Medical Research Skills",
			Slug:        "aipoch-medical-research-skills",
			Category:    "科研 Skill 库",
			Description: "550+ 医学研究 Skill，覆盖证据、方案、数据分析和论文等科研流程。",
			URL:         "https://github.com/aipoch/medical-research-skills",
			Tags:        "Skill,医学研究,证据,数据分析,论文",
			Instruction: "适合构建医学研究 Agent 的流程库。文章提示平台背景不透明且营销偏重，使用前需核验来源和审核结果。",
		},
		{
			Name:        "MedSci Skills",
			Slug:        "medsci-skills",
			Category:    "科研 Skill 库",
			Description: "45 个投稿级临床论文工作流 Skill，由韩国放射科医生结合 Claude Code 和专业经验整理。",
			URL:         "https://github.com/Aperivue/medsci-skills",
			Tags:        "Skill,临床论文,投稿,科研工作流",
			Instruction: "适合临床论文写作、投稿准备和科研工作流参考。边界较清楚，但仍需人工复核医学与期刊要求。",
		},
		{
			Name:        "ToolUniverse",
			Slug:        "tooluniverse",
			Category:    "科研工具库",
			Description: "哈佛医学院 Zitnik 实验室的 1000+ AI scientist 工具底座，覆盖科研和药物方向。",
			URL:         "https://github.com/mims-harvard/ToolUniverse",
			Tags:        "科研工具,药物发现,AI Scientist,ToolUniverse",
			Instruction: "适合科研、药物发现和治疗推理工具编排。可作为 tooluniverse-* 类 Skill 的源头库使用。",
		},
		{
			Name:        "K-Dense Scientific Agent Skills",
			Slug:        "k-dense-scientific-agent-skills",
			Category:    "科研 Skill 库",
			Description: "约 140 个科研 Agent Skill，偏科研场景，社区热度高但需安全扫描。",
			URL:         "https://github.com/K-Dense-AI/scientific-agent-skills",
			Tags:        "Skill,科研Agent,科学工作流,安全扫描",
			Instruction: "适合科研 Agent 能力参考。文章提示 README 自身要求安装前做安全扫描，社区 Skill 不应直接无审查启用。",
		},
		{
			Name:        "Universal Biomedical Skills",
			Slug:        "universal-biomedical-skills",
			Category:    "科研 Skill 库",
			Description: "1207 个生命科学和临床方向 Skill，覆盖广、来源可溯，但分类较散。",
			URL:         "https://github.com/mdbabumiamssm/LLMs-Universal-Life-Science-and-Clinical-Skills-",
			Tags:        "Skill,生命科学,临床,生物医学,资料库",
			Instruction: "适合作为广覆盖资料库收集。文章提示其中混有通用技能、单人新项目且分类较乱，启用前需筛选。",
		},
	}
}
