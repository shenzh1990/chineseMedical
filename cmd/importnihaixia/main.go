package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"chinese-medical/internal/config"
	"chinese-medical/internal/database"
)

const sourceRepo = "https://github.com/shenzh1990/nihaixia"

type nihaixiaSkill struct {
	Name        string
	Slug        string
	Category    string
	Description string
	Tags        string
	Path        string
	Content     string
}

func main() {
	if err := run(); err != nil {
		slog.Error("import nihaixia skills failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	repoPath := "/tmp/nihaixia"
	if len(os.Args) > 1 {
		repoPath = os.Args[1]
	}
	if _, err := os.Stat(filepath.Join(repoPath, "SKILL.md")); err != nil {
		return fmt.Errorf("nihaixia repo path must contain SKILL.md: %w", err)
	}

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

	skills, err := loadNihaixiaSkills(repoPath)
	if err != nil {
		return err
	}

	imported := 0
	for _, item := range skills {
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
			return fmt.Errorf("upsert nihaixia skill %s: %w", item.Slug, err)
		}
		imported++
	}

	var total int64
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM ai_skills WHERE slug LIKE 'nihaixia-%' OR slug = 'nihaixia'`).Scan(&total); err != nil {
		return fmt.Errorf("count nihaixia skills: %w", err)
	}

	fmt.Printf("imported nihaixia skills: %d\n", imported)
	fmt.Printf("nihaixia skillhub rows: %d\n", total)
	return nil
}

func loadNihaixiaSkills(repoPath string) ([]nihaixiaSkill, error) {
	defs := []struct {
		path        string
		name        string
		slug        string
		category    string
		description string
		tags        string
	}{
		{"SKILL.md", "倪海厦经方中医总入口", "nihaixia", "中医经方 Skill", "倪海厦经方中医 AI 总入口，覆盖六经辨证、经方选药、本草、针灸、医案和表达风格。", "倪海厦,经方,六经辨证,伤寒论,金匮要略,中医"},
		{"expression_style.md", "倪海厦口述表达风格", "nihaixia-expression-style", "中医表达 Skill", "提炼倪海厦口语节奏、表达 DNA 和回答风格，用于让回复更贴近原讲课风格。", "倪海厦,表达风格,口述,教学"},
		{"distilled_cases.md", "倪海厦医案结构化索引", "nihaixia-distilled-cases", "中医医案 Skill", "849 个医案的结构化索引，按疾病、六经、方剂和疗效检索。", "倪海厦,医案,结构化索引,疾病检索"},
		{"modules/01_shanghan_sun.md", "倪海厦伤寒论太阳病", "nihaixia-shanghan-sun", "中医经典 Skill", "伤寒论太阳病篇条文 1-129，适合太阳表证、感冒发烧和桂枝汤、麻黄汤等方剂选择。", "倪海厦,伤寒论,太阳病,桂枝汤,麻黄汤"},
		{"modules/02_shanghan_other.md", "倪海厦伤寒论五经辨证", "nihaixia-shanghan-other", "中医经典 Skill", "阳明、少阳、太阴、少阴、厥阴五篇总结，适合里证、半表半里证与阴证辨证。", "倪海厦,伤寒论,阳明,少阳,太阴,少阴,厥阴"},
		{"modules/03_yian.md", "倪海厦综合医案库", "nihaixia-yian", "中医医案 Skill", "849 个医案精选，按日期、疾病、六经、方剂和疗效呈现，用于查找相似临床思路。", "倪海厦,医案,临床案例,六经,方剂"},
		{"modules/04_jingui.md", "倪海厦金匮要略", "nihaixia-jingui", "中医经典 Skill", "金匮要略 23 篇完整解读，适合杂病辨证和金匮方剂查询。", "倪海厦,金匮要略,杂病,经方"},
		{"modules/05_huangdi_neijing.md", "倪海厦黄帝内经精要", "nihaixia-huangdi-neijing", "中医经典 Skill", "黄帝内经 71 篇蒸馏内容，适合中医基础理论、脏腑经络和养生原则。", "倪海厦,黄帝内经,中医理论,脏腑,经络"},
		{"modules/06_liangdong.md", "倪海厦梁冬对话", "nihaixia-liangdong", "中医访谈 Skill", "梁冬对话 7 期精华，适合现代饮食养生、西医批评、日常健康观念等话题。", "倪海厦,梁冬,访谈,饮食养生"},
		{"modules/07_bimen_hantang.md", "倪海厦闭门课与汉唐文章", "nihaixia-bimen-hantang", "中医重病 Skill", "七大重病闭门课与汉唐文章，包含血癌、乳癌、脑瘤等专题思路。", "倪海厦,闭门课,汉唐,重病,癌症"},
		{"modules/08_huangdi_detail.md", "倪海厦黄帝内经详解", "nihaixia-huangdi-detail", "中医经典 Skill", "黄帝内经讲义完整版蒸馏，适合深入内经理论、四时、脏象和病机分析。", "倪海厦,黄帝内经,内经详解,脏象"},
		{"modules/09_zhenjiu_bencao.md", "倪海厦针灸本草天纪", "nihaixia-zhenjiu-bencao", "中医针药 Skill", "针灸教程、神农本草经 345 种、药性理论和天纪内容，适合穴位、药性、五味归经查询。", "倪海厦,针灸,神农本草经,药性,天纪"},
		{"cases/01_cancer.md", "倪海厦癌症医案", "nihaixia-cases-cancer", "中医医案 Skill", "147 个癌症医案，覆盖肝癌、乳癌、肺癌、脑瘤、血癌、淋巴癌等。", "倪海厦,医案,癌症,肿瘤"},
		{"cases/02_cardiovascular.md", "倪海厦心血管医案", "nihaixia-cases-cardiovascular", "中医医案 Skill", "22 个心血管医案，覆盖心脏病、高血压、中风、动脉阻塞等。", "倪海厦,医案,心血管,高血压,中风"},
		{"cases/03_metabolic.md", "倪海厦代谢病医案", "nihaixia-cases-metabolic", "中医医案 Skill", "12 个代谢病医案，覆盖糖尿病、肾衰竭、腹水、肝硬化等。", "倪海厦,医案,糖尿病,肾衰竭,代谢病"},
		{"cases/04_autoimmune.md", "倪海厦自身免疫医案", "nihaixia-cases-autoimmune", "中医医案 Skill", "自身免疫医案，覆盖类风湿、红斑狼疮、风湿等。", "倪海厦,医案,自身免疫,类风湿,红斑狼疮"},
		{"cases/05_neurological.md", "倪海厦神经精神医案", "nihaixia-cases-neurological", "中医医案 Skill", "神经精神医案，覆盖癫痫、帕金森、忧郁等。", "倪海厦,医案,癫痫,帕金森,神经精神"},
		{"cases/06_other.md", "倪海厦其他杂病医案", "nihaixia-cases-other", "中医医案 Skill", "其他杂病医案，覆盖痛经、鼻炎、不孕、皮肤病等。", "倪海厦,医案,杂病,痛经,鼻炎,不孕,皮肤病"},
	}

	skills := make([]nihaixiaSkill, 0, len(defs))
	for _, def := range defs {
		content, err := os.ReadFile(filepath.Join(repoPath, def.path))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", def.path, err)
		}
		skills = append(skills, nihaixiaSkill{
			Name:        def.name,
			Slug:        def.slug,
			Category:    def.category,
			Description: def.description,
			Tags:        def.tags,
			Path:        def.path,
			Content:     string(content),
		})
	}
	return skills, nil
}

func instructionFor(item nihaixiaSkill) string {
	return fmt.Sprintf(`来源仓库：%s
来源文件：%s

适用范围：
%s

使用规则：
1. 仅作为中医知识、经方思路、经典学习和医案参考，不替代医生诊疗。
2. 回答时必须提示：涉及急症、重病、孕产妇、儿童、老人、慢病用药、肿瘤、心脑血管、肾衰竭等情况，应线下就医并由专业医师判断。
3. 倪海厦相关观点应标明为“倪海厦体系/讲义中的观点”，避免包装为现代医学定论。
4. 不给出保证疗效、确诊结论、处方剂量或停用现有治疗的建议。
5. 若用于问答上下文，优先按本 Skill 的主题范围回答；超出范围时说明需要切换到其他 Nihaixia 模块或本地知识库。

内容索引摘录：
%s`, sourceRepo, item.Path, item.Description, excerptMarkdown(item.Content, 12000))
}

func excerptMarkdown(content string, maxRunes int) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = stripFrontMatter(content)
	lines := strings.Split(content, "\n")
	picked := make([]string, 0, 200)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if len(picked) > 0 && picked[len(picked)-1] != "" {
				picked = append(picked, "")
			}
			continue
		}
		if strings.HasPrefix(trimmed, "#") ||
			strings.HasPrefix(trimmed, ">") ||
			strings.HasPrefix(trimmed, "|") ||
			strings.HasPrefix(trimmed, "- ") ||
			strings.HasPrefix(trimmed, "* ") ||
			looksNumbered(trimmed) {
			picked = append(picked, line)
		}
		if runeLen(strings.Join(picked, "\n")) >= maxRunes {
			break
		}
	}
	if len(picked) == 0 {
		picked = strings.Split(content, "\n")
	}
	excerpt := strings.TrimSpace(strings.Join(picked, "\n"))
	runes := []rune(excerpt)
	if len(runes) > maxRunes {
		excerpt = string(runes[:maxRunes]) + "\n..."
	}
	return excerpt
}

func stripFrontMatter(content string) string {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return content
	}
	parts := strings.SplitN(content, "---", 3)
	if len(parts) == 3 {
		return strings.TrimSpace(parts[2])
	}
	return content
}

var numberedPrefix = regexp.MustCompile(`^\d+[\.\)、]`)

func looksNumbered(line string) bool {
	if numberedPrefix.MatchString(line) {
		return true
	}
	r, _ := utf8FirstRune(line)
	return unicode.IsDigit(r)
}

func utf8FirstRune(value string) (rune, int) {
	for i, r := range value {
		return r, i
	}
	return 0, 0
}

func runeLen(value string) int {
	return len([]rune(value))
}
