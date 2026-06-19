package references

import "strings"

func SourceLabel(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "openalex":
		return "OpenAlex（开放学术索引）"
	case "doaj":
		return "DOAJ（开放获取期刊）"
	case "semantic_scholar":
		return "Semantic Scholar（学术搜索源）"
	case "crossref":
		return "Crossref（DOI/出版元数据）"
	case "arxiv":
		return "arXiv（开放预印本仓储）"
	case "pubmed":
		return "PubMed（生物医学文献库）"
	case "bibtex":
		return "本地 BibTeX 导入"
	case "ris":
		return "本地 RIS 导入"
	case "csv_export":
		return "本地 CSV 导入"
	case "":
		return "未知来源"
	default:
		return strings.TrimSpace(source)
	}
}

func ReliabilityLabel(reliability string) string {
	switch strings.ToLower(strings.TrimSpace(reliability)) {
	case "official_api":
		return "官方公开接口"
	case "repository":
		return "开放仓储"
	case "crossref_metadata":
		return "Crossref 出版元数据"
	case "user_export":
		return "用户导入的文献导出文件"
	case "unverified_link":
		return "公开链接，建议人工核验"
	case "":
		return ""
	default:
		return strings.TrimSpace(reliability)
	}
}

func AvailabilityLabel(availability string) string {
	switch strings.ToLower(strings.TrimSpace(availability)) {
	case "open_access":
		return "开放获取"
	case "landing_page":
		return "可访问文献落地页"
	case "doi_landing":
		return "可通过 DOI 访问落地页"
	case "subscription_required":
		return "可能需要订阅或机构权限"
	case "unknown":
		return "可获取性未知"
	case "":
		return ""
	default:
		return strings.TrimSpace(availability)
	}
}
