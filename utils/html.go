package utils

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	htmpl "html/template"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	ttmpl "text/template"
	"time"
)

const (
	DefaultHTMLFileName     = "result.html"
	DefaultHTMLConfig       = "result.yaml"
	DefaultHistoryFileName  = "result_history.json"
	defaultHistoryHours     = 72
	defaultTopLimit         = 10
)

// 可配置的输出路径（在main.go中设置）
var (
	HTMLOutputPath    = DefaultHTMLFileName
	HistoryOutputPath = DefaultHistoryFileName
	GlobalHTMLConfig  HTMLConfig // 由main.go设置
)

type htmlConfig struct {
	Environment   string `yaml:"environment"`
	HistoryHours  int    `yaml:"history_hours"`
	VMessTemplate string `yaml:"vmess_template"`
}

// HTMLConfig 是对外导出的配置类型
type HTMLConfig struct {
	Environment   string
	HistoryHours  int
	VMessTemplate string
}

type htmlPageData struct {
	Count            int
	TestedAt         string
	Environment      string
	Rows             []htmlRow
	HasVMessTemplate bool
	ConfigNotice     string
}

type htmlRow struct {
	Rank          int
	Data          []string
	VMessLink     string
	HasVMessLink  bool
	VMessDisabled bool
}

type vmessTemplateData struct {
	IP           string
	Address      string
	Rank         int
	Sended       int
	Received     int
	LossRate     float64
	LossRateText string
	DelayMS      float64
	DelayText    string
	SpeedMBps    float64
	SpeedText    string
	Colo         string
	Timestamp    string
}

type historyRecord struct {
	IP            string    `json:"ip"`
	Sended        int       `json:"sended"`
	Received      int       `json:"received"`
	DelayMS       float64   `json:"delay_ms"`
	DownloadSpeed float64   `json:"download_speed"`
	Colo          string    `json:"colo"`
	TestedAt      time.Time `json:"tested_at"`
	Environment   string    `json:"environment"`
}

type renderRow struct {
	Rank          int
	IP            string
	TestedAt      string
	Sended        string
	Received      string
	LossRate      string
	DelayText     string
	SpeedText     string
	Colo          string
	VMessLink     string
	VMessDisabled bool
}

type metaItem struct {
	Label string
	Value string
}

type sectionData struct {
	ID           string
	Title        string
	Subtitle     string
	MetaItems    []metaItem
	RowsHTML     htmpl.HTML
	EmptyMessage string
	Active       bool
}

func loadHTMLConfig() (htmlConfig, error) {
	// 尝试读取instance/config.yaml作为后备，但优先使用GlobalHTMLConfig
	content, err := os.ReadFile("instance/config.yaml")
	if err != nil {
		if os.IsNotExist(err) {
			return htmlConfig{}, nil
		}
		return htmlConfig{}, err
	}

	return parseHTMLConfig(string(content)), nil
}

func historyWindowFromHours(hours int) time.Duration {
	if hours <= 0 {
		hours = defaultHistoryHours
	}
	return time.Duration(hours) * time.Hour
}

func parseHTMLConfig(raw string) htmlConfig {
	var cfg htmlConfig
	lines := strings.Split(raw, "\n")
	blockKey := ""
	blockLines := make([]string, 0)

	flushBlock := func() {
		if blockKey == "vmess_template" {
			cfg.VMessTemplate = dedentBlock(blockLines)
		}
		blockKey = ""
		blockLines = blockLines[:0]
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if blockKey != "" {
			if trimmed == "" || strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
				blockLines = append(blockLines, line)
				continue
			}
			flushBlock()
			i--
			continue
		}

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		key, value, found := strings.Cut(trimmed, ":")
		if !found {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		switch key {
		case "environment":
			cfg.Environment = parseHTMLScalar(value)
		case "history_hours":
			if parsed, err := strconv.Atoi(parseHTMLScalar(value)); err == nil {
				cfg.HistoryHours = parsed
			}
		case "vmess_template":
			if value == "|" || value == "|-" || value == "|+" || value == "" {
				blockKey = key
				blockLines = blockLines[:0]
				continue
			}
			cfg.VMessTemplate = parseHTMLScalar(value)
		}
	}

	if blockKey != "" {
		flushBlock()
	}

	return cfg
}

func dedentBlock(lines []string) string {
	if len(lines) == 0 {
		return ""
	}

	start := 0
	end := len(lines)
	for start < end && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	if start >= end {
		return ""
	}

	indent := -1
	for i := start; i < end; i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		leading := len(line) - len(strings.TrimLeft(line, " \t"))
		if indent < 0 || leading < indent {
			indent = leading
		}
	}
	if indent < 0 {
		indent = 0
	}

	var builder strings.Builder
	for i := start; i < end; i++ {
		if i > start {
			builder.WriteByte('\n')
		}
		line := lines[i]
		if len(line) > indent {
			builder.WriteString(line[indent:])
		} else {
			builder.WriteString(strings.TrimLeft(line, " \t"))
		}
	}
	return strings.TrimRight(builder.String(), "\n")
}

func parseHTMLScalar(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if unquoted, err := strconv.Unquote(value); err == nil {
		return unquoted
	}
	return strings.Trim(value, "\"'")
}

func compileVMessTemplate(raw string) *ttmpl.Template {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	template, err := ttmpl.New("vmess").Parse(raw)
	if err != nil {
		log.Printf("[警告] 解析 vmess_template 失败：%v", err)
		return nil
	}
	return template
}

func buildVMessLink(template *ttmpl.Template, record historyRecord, rank int) (string, bool) {
	if template == nil {
		return "", false
	}

	var builder bytes.Buffer
	data := vmessTemplateData{
		IP:           record.IP,
		Address:      record.IP,
		Rank:         rank,
		Sended:       record.Sended,
		Received:     record.Received,
		LossRate:     recordLossRate(record),
		LossRateText: fmt.Sprintf("%.2f", recordLossRate(record)),
		DelayMS:      record.DelayMS,
		DelayText:    fmt.Sprintf("%.2f", record.DelayMS),
		SpeedMBps:    record.DownloadSpeed / 1024 / 1024,
		SpeedText:    fmt.Sprintf("%.2f", record.DownloadSpeed/1024/1024),
		Colo:         record.Colo,
		Timestamp:    record.TestedAt.Format("2006-01-02 15:04:05"),
	}
	if err := template.Execute(&builder, data); err != nil {
		log.Printf("[警告] 渲染 vmess_template 失败：%v", err)
		return "", false
	}

	payload := strings.TrimSpace(builder.String())
	if payload == "" {
		return "", false
	}
	return "vmess://" + base64.StdEncoding.EncodeToString([]byte(payload)), true
}

func buildRow(record historyRecord, rank int, template *ttmpl.Template) renderRow {
	vmessLink, hasVMessLink := buildVMessLink(template, record, rank)
	return recordToRow(record, rank, vmessLink, hasVMessLink)
}

func recordFromIPData(data CloudflareIPData, testedAt time.Time, environment string) historyRecord {
	ip := ""
	if data.IP != nil {
		ip = data.IP.String()
	}
	return historyRecord{
		IP:            ip,
		Sended:        data.Sended,
		Received:      data.Received,
		DelayMS:       data.Delay.Seconds() * 1000,
		DownloadSpeed: data.DownloadSpeed,
		Colo:          data.Colo,
		TestedAt:      testedAt,
		Environment:   environment,
	}
}

func recordLossRate(record historyRecord) float64 {
	if record.Sended <= 0 {
		return 0
	}
	return float64(record.Sended-record.Received) / float64(record.Sended)
}

func recordToRow(record historyRecord, rank int, vmessLink string, hasVMessLink bool) renderRow {
	colo := strings.TrimSpace(record.Colo)
	if colo == "" {
		colo = "N/A"
	}
	return renderRow{
		Rank:          rank,
		IP:            record.IP,
		TestedAt:      record.TestedAt.Format("2006-01-02 15:04:05"),
		Sended:        strconv.Itoa(record.Sended),
		Received:      strconv.Itoa(record.Received),
		LossRate:      fmt.Sprintf("%.2f", recordLossRate(record)),
		DelayText:     fmt.Sprintf("%.2f", record.DelayMS),
		SpeedText:     fmt.Sprintf("%.2f", record.DownloadSpeed/1024/1024),
		Colo:          colo,
		VMessLink:     vmessLink,
		VMessDisabled: !hasVMessLink,
	}
}

func loadHistoryStore() ([]historyRecord, error) {
	content, err := os.ReadFile(HistoryOutputPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(bytes.TrimSpace(content)) == 0 {
		return nil, nil
	}

	var records []historyRecord
	if err := json.Unmarshal(content, &records); err != nil {
		return nil, err
	}
	return records, nil
}

func saveHistoryStore(records []historyRecord) error {
	content, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')
	return os.WriteFile(HistoryOutputPath, content, 0644)
}

func mergeHistoryStore(currentRecords []historyRecord, historyWindow time.Duration, testedAt time.Time) ([]historyRecord, error) {
	existingRecords, err := loadHistoryStore()
	if err != nil {
		return nil, err
	}

	cutoff := testedAt.Add(-historyWindow)
	recordMap := make(map[string]historyRecord, len(existingRecords)+len(currentRecords))
	for _, record := range existingRecords {
		if record.TestedAt.Before(cutoff) {
			continue
		}
		if strings.TrimSpace(record.IP) == "" {
			continue
		}
		recordMap[record.IP] = record
	}
	for _, record := range currentRecords {
		if record.TestedAt.Before(cutoff) {
			continue
		}
		if strings.TrimSpace(record.IP) == "" {
			continue
		}
		if oldRecord, ok := recordMap[record.IP]; !ok || oldRecord.TestedAt.Before(record.TestedAt) {
			recordMap[record.IP] = record
		}
	}

	records := mapToRecords(recordMap)
	sortRecordsBySpeed(records)
	if len(records) > defaultTopLimit {
		records = records[:defaultTopLimit]
	}
	if err := saveHistoryStore(records); err != nil {
		return records, err
	}
	return records, nil
}

func mapToRecords(recordMap map[string]historyRecord) []historyRecord {
	records := make([]historyRecord, 0, len(recordMap))
	for _, record := range recordMap {
		records = append(records, record)
	}
	return records
}

func sortRecordsBySpeed(records []historyRecord) {
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].DownloadSpeed != records[j].DownloadSpeed {
			return records[i].DownloadSpeed > records[j].DownloadSpeed
		}
		if !records[i].TestedAt.Equal(records[j].TestedAt) {
			return records[i].TestedAt.After(records[j].TestedAt)
		}
		return records[i].IP < records[j].IP
	})
}

func topCurrentRecords(data []CloudflareIPData, testedAt time.Time, environment string, limit int) []historyRecord {
	records := make([]historyRecord, 0, len(data))
	for _, item := range data {
		records = append(records, recordFromIPData(item, testedAt, environment))
	}
	sortRecordsBySpeed(records)
	if limit > 0 && len(records) > limit {
		records = records[:limit]
	}
	return records
}

func topHistoryRecords(records []historyRecord, limit int, testedAt time.Time, historyWindow time.Duration) []historyRecord {
	cutoff := testedAt.Add(-historyWindow)
	filtered := make([]historyRecord, 0, len(records))
	for _, record := range records {
		if record.TestedAt.Before(cutoff) {
			continue
		}
		filtered = append(filtered, record)
	}
	sortRecordsBySpeed(filtered)
	if limit > 0 && len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered
}

func renderRowsHTML(records []historyRecord, template *ttmpl.Template) htmpl.HTML {
	if len(records) == 0 {
		return htmpl.HTML(`<tr><td class="empty-row" colspan="10">暂无结果</td></tr>`)
	}

	var builder strings.Builder
	for index, record := range records {
		row := buildRow(record, index+1, template)
		vmessLink := htmpl.HTMLEscapeString(row.VMessLink)
		vmessDisabled := ""
		if row.VMessDisabled {
			vmessDisabled = " disabled"
		}
		builder.WriteString("<tr>")
		builder.WriteString(fmt.Sprintf("<td class=\"rank\">%d</td>", row.Rank))
		builder.WriteString(fmt.Sprintf("<td>%s</td>", htmpl.HTMLEscapeString(row.IP)))
		builder.WriteString(fmt.Sprintf("<td>%s</td>", htmpl.HTMLEscapeString(row.TestedAt)))
		builder.WriteString(fmt.Sprintf("<td>%s</td>", htmpl.HTMLEscapeString(row.Sended)))
		builder.WriteString(fmt.Sprintf("<td>%s</td>", htmpl.HTMLEscapeString(row.Received)))
		builder.WriteString(fmt.Sprintf("<td>%s</td>", htmpl.HTMLEscapeString(row.LossRate)))
		builder.WriteString(fmt.Sprintf("<td>%s</td>", htmpl.HTMLEscapeString(row.DelayText)))
		builder.WriteString(fmt.Sprintf("<td>%s</td>", htmpl.HTMLEscapeString(row.SpeedText)))
		builder.WriteString(fmt.Sprintf("<td>%s</td>", htmpl.HTMLEscapeString(row.Colo)))
		builder.WriteString("<td class=\"action-cell\">")
		builder.WriteString(fmt.Sprintf("<button type=\"button\" class=\"copy-btn icon-btn\" data-vmess=\"%s\" onclick=\"copyVmess(this)\" aria-label=\"复制 VMess 链接\"%s><svg viewBox=\"0 0 24 24\" aria-hidden=\"true\"><path d=\"M16 1H6a2 2 0 0 0-2 2v12h2V3h10V1zm3 4H10a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h9a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2zm0 16H10V7h9v14z\"/></svg></button>", vmessLink, vmessDisabled))
		builder.WriteString(fmt.Sprintf("<button type=\"button\" class=\"copy-btn icon-btn qr-btn\" data-vmess=\"%s\" onclick=\"showQr(this)\" aria-label=\"查看订阅二维码\"%s><svg viewBox=\"0 0 24 24\" aria-hidden=\"true\"><path d=\"M3 3h8v8H3V3zm2 2v4h4V5H5zm0 10h4v4H5v-4zm10-10h6v6h-6V5zm2 2v2h2V7h-2zM13 13h4v4h-4v-4zm6 6h2v2h-2v-2zm-6-6h2v2h-2v-2zm4 4h2v2h-2v-2zm0 4h2v2h-2v-2zm-8-4h2v2h-2v-2zm-2 4h2v2H7v-2z\"/></svg></button>", vmessLink, vmessDisabled))
		builder.WriteString("</td></tr>")
	}
	return htmpl.HTML(builder.String())
}

func renderSectionHTML(data sectionData) htmpl.HTML {
	var builder strings.Builder
	className := "result-section"

	builder.WriteString(fmt.Sprintf("<section class=\"%s\" id=\"view-%s\">", className, htmpl.HTMLEscapeString(data.ID)))
	builder.WriteString("<div class=\"hero compact\">")
	builder.WriteString(fmt.Sprintf("<h2>%s</h2>", htmpl.HTMLEscapeString(data.Title)))
	builder.WriteString(fmt.Sprintf("<p class=\"sub\">%s</p>", htmpl.HTMLEscapeString(data.Subtitle)))
	builder.WriteString("</div>")
	builder.WriteString("<div class=\"meta\">")
	for _, item := range data.MetaItems {
		builder.WriteString("<div class=\"meta-item\">")
		builder.WriteString(fmt.Sprintf("<div class=\"meta-label\">%s</div>", htmpl.HTMLEscapeString(item.Label)))
		builder.WriteString(fmt.Sprintf("<div class=\"meta-value\">%s</div>", htmpl.HTMLEscapeString(item.Value)))
		builder.WriteString("</div>")
	}
	builder.WriteString("</div>")
	builder.WriteString("<div class=\"table-wrap\"><table><thead><tr>")
	builder.WriteString("<th>#</th><th>IP 地址</th><th>测速时间</th><th>已发送</th><th>已接收</th><th>丢包率</th><th>平均延迟(ms)</th><th>下载速度(MB/s)</th><th>地区码</th><th>VMess</th>")
	builder.WriteString("</tr></thead><tbody>")
	builder.WriteString(string(data.RowsHTML))
	builder.WriteString("</tbody></table></div>")
	builder.WriteString("</section>")
	return htmpl.HTML(builder.String())
}

// ExportTopHTML 会在测速结束后导出前 10 个结果为 HTML 表格。
func ExportTopHTML(data []CloudflareIPData) {
	// 使用从main.go加载的全局配置
	cfg := htmlConfig{
		Environment:   GlobalHTMLConfig.Environment,
		HistoryHours:  GlobalHTMLConfig.HistoryHours,
		VMessTemplate: GlobalHTMLConfig.VMessTemplate,
	}
	vmessTpl := compileVMessTemplate(cfg.VMessTemplate)

	pageTemplate := htmpl.Must(htmpl.New("page").Parse(`<!doctype html>
}
<html lang="zh-CN">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>CloudflareSpeedTest 结果</title>
	<style>
		:root {
			color-scheme: light;
			--bg: #f7f9fc;
			--card: #ffffff;
			--line: #e4e9f0;
			--text: #1f2937;
			--text-soft: #667085;
			--accent: #0b6ef9;
			--accent-soft: #eaf2ff;
		}
		* { box-sizing: border-box; }
		body {
			margin: 0;
			font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif;
			background: radial-gradient(circle at top right, #e9f1ff, var(--bg) 30%), var(--bg);
			color: var(--text);
		}
		.wrap {
			max-width: 1280px;
			margin: 36px auto 42px;
			padding: 0 16px;
		}
		.card {
			background: var(--card);
			border: 1px solid var(--line);
			border-radius: 18px;
			box-shadow: 0 10px 30px rgba(16, 24, 40, 0.08);
			overflow: hidden;
		}
		.hero {
			padding: 22px 24px 16px;
			border-bottom: 1px solid var(--line);
			background: linear-gradient(180deg, #ffffff, #f8fbff);
		}
		.hero.compact {
			border-bottom: 0;
			padding-bottom: 0;
			background: transparent;
		}
		h1, h2 {
			margin: 0;
			line-height: 1.2;
		}
		h1 { font-size: 24px; }
		h2 { font-size: 20px; }
		.sub {
			margin: 10px 0 0;
			color: var(--text-soft);
			font-size: 14px;
			line-height: 1.6;
		}
		.tabs {
			display: flex;
			gap: 10px;
			padding: 20px 24px 0;
			flex-wrap: wrap;
		}
		.tab-btn {
			appearance: none;
			border: 1px solid var(--line);
			background: #fff;
			color: var(--text);
			border-radius: 999px;
			padding: 10px 16px;
			font-size: 14px;
			font-weight: 700;
			cursor: pointer;
		}
		.tab-btn.active {
			background: var(--accent);
			color: #fff;
			border-color: var(--accent);
			box-shadow: 0 8px 18px rgba(11, 110, 249, 0.18);
		}
		.view {
			display: none;
			padding-bottom: 12px;
		}
		.view.is-active {
			display: block;
		}
		.meta {
			display: grid;
			grid-template-columns: repeat(3, minmax(0, 1fr));
			gap: 12px;
			padding: 18px 24px 0;
		}
		.meta-item {
			border: 1px solid var(--line);
			border-radius: 14px;
			background: #fbfdff;
			padding: 14px 16px;
		}
		.meta-label {
			font-size: 12px;
			color: var(--text-soft);
			margin-bottom: 6px;
		}
		.meta-value {
			font-size: 15px;
			font-weight: 600;
			word-break: break-word;
		}
		.table-wrap {
			width: 100%;
			overflow-x: auto;
			padding: 18px 24px 6px;
		}
		table {
			width: 100%;
			border-collapse: collapse;
			min-width: 1000px;
			overflow: hidden;
			border-radius: 14px;
		}
		th, td {
			padding: 12px 14px;
			border-top: 1px solid var(--line);
			text-align: left;
			font-size: 14px;
			white-space: nowrap;
			vertical-align: middle;
		}
		th {
			background: #eef4ff;
			color: #0f172a;
			border-top: none;
			font-weight: 700;
		}
		tr:nth-child(even) td { background: #fbfdff; }
		.rank { color: var(--accent); font-weight: 700; }
		.copy-btn, .sub-btn {
			appearance: none;
			border: 0;
			border-radius: 999px;
			padding: 9px 14px;
			font-size: 13px;
			font-weight: 700;
			cursor: pointer;
			transition: transform 0.16s ease, box-shadow 0.16s ease, background 0.16s ease;
			text-decoration: none;
			display: inline-flex;
			align-items: center;
			justify-content: center;
			gap: 8px;
		}
		.copy-btn.icon-btn {
			width: 38px;
			height: 38px;
			padding: 0;
			border-radius: 12px;
			position: relative;
		}
		.copy-btn.icon-btn svg {
			width: 18px;
			height: 18px;
			fill: currentColor;
			display: block;
		}
		.copy-btn.icon-btn.qr-btn::after {
			content: "";
			position: absolute;
			inset: 9px;
			border: 2px solid currentColor;
			border-radius: 5px;
			opacity: 0.35;
		}
		.copy-btn {
			background: var(--accent);
			color: #fff;
			box-shadow: 0 8px 18px rgba(11, 110, 249, 0.18);
		}
		.copy-btn:hover, .sub-btn:hover { transform: translateY(-1px); }
		.copy-btn:disabled {
			cursor: not-allowed;
			background: #94a3b8;
			box-shadow: none;
		}
		.copy-btn.is-copied { background: #16a34a; }
		.action-cell {
			min-width: 120px;
			display: flex;
			gap: 8px;
			align-items: center;
		}
		.footer {
			padding: 8px 24px 24px;
		}
		.footer-note {
			margin: 0 0 12px;
			color: var(--text-soft);
			font-size: 13px;
			line-height: 1.6;
		}
		.footer-actions {
			display: flex;
			flex-wrap: wrap;
			gap: 12px;
			align-items: center;
		}
		.sub-btn {
			background: var(--accent-soft);
			color: var(--accent);
			border: 1px solid #cfe0ff;
		}
		.notice {
			margin: 10px 24px 0;
			padding: 12px 14px;
			border-radius: 12px;
			background: #fff7ed;
			color: #9a3412;
			border: 1px solid #fed7aa;
			font-size: 13px;
			line-height: 1.6;
		}
		.empty-row {
			text-align: center;
			color: var(--text-soft);
			padding: 24px 14px;
		}
		@media (max-width: 900px) {
			.meta { grid-template-columns: 1fr; }
			.action-cell { min-width: 106px; }
		}
		.modal {
			display: none;
			position: fixed;
			inset: 0;
			background: rgba(15, 23, 42, 0.58);
			backdrop-filter: blur(8px);
			align-items: center;
			justify-content: center;
			padding: 16px;
			z-index: 50;
		}
		.modal.is-open { display: flex; }
		.modal-card {
			width: min(360px, 100%);
			background: #fff;
			border-radius: 18px;
			padding: 18px;
			box-shadow: 0 20px 40px rgba(0, 0, 0, 0.18);
		}
		.modal-head {
			display: flex;
			align-items: center;
			justify-content: space-between;
			gap: 12px;
			margin-bottom: 14px;
		}
		.modal-title {
			font-size: 16px;
			font-weight: 700;
		}
		.modal-close {
			border: 0;
			background: #eef4ff;
			color: #0f172a;
			width: 34px;
			height: 34px;
			border-radius: 999px;
			cursor: pointer;
			font-size: 20px;
			line-height: 1;
		}
		.modal-qr {
			display: block;
			width: 240px;
			height: 240px;
			margin: 0 auto;
			border-radius: 14px;
			border: 1px solid #e5e7eb;
			background: #fff;
		}
		.modal-desc {
			margin: 14px 0 0;
			font-size: 13px;
			color: var(--text-soft);
			line-height: 1.6;
			word-break: break-all;
		}
	</style>
</head>
<body>
	<div class="wrap">
		<section class="card">
			<div class="hero">
				<h1>CloudflareSpeedTest 结果</h1>
				<p class="sub">可在“本次结果”和“最近历史结果”之间切换；二维码按钮可直接展示对应 VMess 的订阅二维码。</p>
			</div>

			<div class="tabs">
				<button class="tab-btn active" type="button" data-target="current">本次结果</button>
				<button class="tab-btn" type="button" data-target="history">最近历史结果</button>
			</div>

			<div class="view is-active" id="view-current">{{.CurrentSection}}</div>
			<div class="view" id="view-history">{{.HistorySection}}</div>

			<div class="footer">
				<p class="footer-note">提示：请先复制对应的 Vmess 订阅链接，再到其他格式订阅转换工具中使用；本地历史数据会保存在 {{.HistoryFile}} 中。</p>
				<div class="footer-actions">
					<a class="sub-btn" href="https://subconverters.com/" target="_blank" rel="noreferrer">获取其他格式订阅</a>
				</div>
			</div>
		</section>
	</div>

	<div class="modal" id="qr-modal" aria-hidden="true">
		<div class="modal-card" role="dialog" aria-modal="true" aria-labelledby="qr-title">
			<div class="modal-head">
				<div class="modal-title" id="qr-title">订阅二维码</div>
				<button class="modal-close" type="button" onclick="closeQr()" aria-label="关闭">×</button>
			</div>
			<img class="modal-qr" id="qr-image" alt="订阅二维码" src="">
			<p class="modal-desc" id="qr-text"></p>
		</div>
	</div>

	<script>
		var qrModal = document.getElementById('qr-modal');
		var qrImage = document.getElementById('qr-image');
		var qrLoading = document.getElementById('qr-loading');
		var qrText = document.getElementById('qr-text');

		function copyText(text) {
			if (navigator.clipboard && window.isSecureContext) {
				return navigator.clipboard.writeText(text);
			}
			return new Promise(function(resolve, reject) {
				var textArea = document.createElement('textarea');
				textArea.value = text;
				textArea.style.position = 'fixed';
				textArea.style.opacity = '0';
				document.body.appendChild(textArea);
				textArea.focus();
				textArea.select();
				try {
					var ok = document.execCommand('copy');
					if (ok) {
						resolve();
					} else {
						reject(new Error('copy failed'));
					}
				} catch (error) {
					reject(error);
				} finally {
					document.body.removeChild(textArea);
				}
			});
		}

		function copyVmess(button) {
			var link = button.getAttribute('data-vmess');
			if (!link) {
				alert('请先在 result.yaml 中配置 vmess_template');
				return;
			}

			var originalText = button.textContent;
			copyText(link).then(function() {
				button.textContent = '已复制';
				button.classList.add('is-copied');
				setTimeout(function() {
					button.textContent = originalText;
					button.classList.remove('is-copied');
				}, 1400);
			}).catch(function() {
				alert('复制失败，请手动复制该链接。');
			});
		}

		function showQr(button) {
			var link = button.getAttribute('data-vmess');
			if (!link) {
				alert('请先在 result.yaml 中配置 vmess_template');
				return;
			}
			qrImage.src = 'https://api.qrserver.com/v1/create-qr-code/?size=240x240&data=' + encodeURIComponent(link);
			qrText.textContent = link;
			qrModal.classList.add('is-open');
			qrModal.setAttribute('aria-hidden', 'false');
		}

		function closeQr() {
			qrModal.classList.remove('is-open');
			qrModal.setAttribute('aria-hidden', 'true');
			qrImage.src = '';
			qrText.textContent = '';
		}

		qrModal.addEventListener('click', function(event) {
			if (event.target === qrModal) {
				closeQr();
			}
		});

		document.addEventListener('keydown', function(event) {
			if (event.key === 'Escape') {
				closeQr();
			}
		});

		document.querySelectorAll('.tab-btn').forEach(function(button) {
			button.addEventListener('click', function() {
				var target = button.getAttribute('data-target');
				document.querySelectorAll('.tab-btn').forEach(function(other) {
					other.classList.toggle('active', other === button);
				});
				document.querySelectorAll('.view').forEach(function(view) {
					view.classList.toggle('is-active', view.id === 'view-' + target);
				});
			});
		});
	</script>
</body>
</html>`))

	pageEnvironment := strings.TrimSpace(cfg.Environment)
	if pageEnvironment == "" {
		pageEnvironment = "未配置"
	}
	historyWindow := historyWindowFromHours(cfg.HistoryHours)
	historyHours := int(historyWindow / time.Hour)

	testedAt := time.Now()
	allCurrentRecords := topCurrentRecords(data, testedAt, pageEnvironment, len(data))
	currentRecords := allCurrentRecords
	if len(currentRecords) > 10 {
		currentRecords = currentRecords[:10]
	}
	historyRecords, err := mergeHistoryStore(allCurrentRecords, historyWindow, testedAt)
	if err != nil {
		log.Printf("[警告] 更新本地历史记录失败：%v", err)
		historyRecords = allCurrentRecords
	}
	historyRecords = topHistoryRecords(historyRecords, 10, testedAt, historyWindow)

	currentSection := renderSectionHTML(sectionData{
		ID:       "current",
		Title:    "本次结果",
		Subtitle: "当前测速会话的前 10 个 IP，按速度从快到慢排序。",
		MetaItems: []metaItem{
			{Label: "测速时间", Value: testedAt.Format("2006-01-02 15:04:05")},
			{Label: "测速环境", Value: pageEnvironment},
			{Label: "展示数量", Value: fmt.Sprintf("%d 个 IP", len(currentRecords))},
		},
		RowsHTML:     renderRowsHTML(currentRecords, vmessTpl),
		EmptyMessage: "本次结果为空",
		Active:       true,
	})
	historySection := renderSectionHTML(sectionData{
		ID:       "history",
		Title:    "最近历史结果",
		Subtitle: fmt.Sprintf("从本地历史数据中取出最近 %d 小时的记录，按速度从快到慢排序前 10 个。", historyHours),
		MetaItems: []metaItem{
			{Label: "数据来源", Value: HistoryOutputPath},
			{Label: "时间窗口", Value: fmt.Sprintf("最近 %d 小时", historyHours)},
			{Label: "展示数量", Value: fmt.Sprintf("%d 个 IP", len(historyRecords))},
		},
		RowsHTML:     renderRowsHTML(historyRecords, vmessTpl),
		EmptyMessage: fmt.Sprintf("最近 %d 小时暂无记录", historyHours),
		Active:       false,
	})

	pageTemplate = htmpl.Must(htmpl.New("page").Parse(`<!doctype html>
<html lang="zh-CN">
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>CloudflareSpeedTest 结果</title>
    <style>
        :root {
            color-scheme: light;
            --bg: #f7f9fc;
            --card: #ffffff;
            --line: #e4e9f0;
            --text: #1f2937;
            --text-soft: #667085;
            --accent: #0b6ef9;
            --accent-soft: #eaf2ff;
        }
        * { box-sizing: border-box; }
        body {
            margin: 0;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC", "Microsoft YaHei", sans-serif;
            background: radial-gradient(circle at top right, #e9f1ff, var(--bg) 30%), var(--bg);
            color: var(--text);
        }
        .wrap {
            max-width: 1280px;
            margin: 36px auto 42px;
            padding: 0 16px;
        }
        .card {
            background: var(--card);
            border: 1px solid var(--line);
            border-radius: 18px;
            box-shadow: 0 10px 30px rgba(16, 24, 40, 0.08);
            overflow: hidden;
        }
        .hero {
            padding: 22px 24px 16px;
            border-bottom: 1px solid var(--line);
            background: linear-gradient(180deg, #ffffff, #f8fbff);
        }
        .hero.compact {
            border-bottom: 0;
            padding-bottom: 0;
            background: transparent;
        }
        h1, h2 {
            margin: 0;
            line-height: 1.2;
        }
        h1 { font-size: 24px; }
        h2 { font-size: 20px; }
        .sub {
            margin: 10px 0 0;
            color: var(--text-soft);
            font-size: 14px;
            line-height: 1.6;
        }
        .tabs {
            display: flex;
            gap: 10px;
            padding: 20px 24px 0;
            flex-wrap: wrap;
        }
        .tab-btn {
            appearance: none;
            border: 1px solid var(--line);
            background: #fff;
            color: var(--text);
            border-radius: 999px;
            padding: 10px 16px;
            font-size: 14px;
            font-weight: 700;
            cursor: pointer;
        }
		.tab-btn.active {
			background: var(--accent);
			color: #fff;
			border-color: var(--accent);
			box-shadow: 0 8px 18px rgba(11, 110, 249, 0.18);
		}
		.result-section {
			padding-bottom: 12px;
		}
		.result-section + .result-section {
			margin-top: 6px;
			border-top: 1px dashed var(--line);
		}
        .meta {
            display: grid;
            grid-template-columns: repeat(3, minmax(0, 1fr));
            gap: 12px;
            padding: 18px 24px 0;
        }
        .meta-item {
            border: 1px solid var(--line);
            border-radius: 14px;
            background: #fbfdff;
            padding: 14px 16px;
        }
        .meta-label {
            font-size: 12px;
            color: var(--text-soft);
            margin-bottom: 6px;
        }
        .meta-value {
            font-size: 15px;
            font-weight: 600;
            word-break: break-word;
        }
        .table-wrap {
            width: 100%;
            overflow-x: auto;
            padding: 18px 24px 6px;
        }
        table {
            width: 100%;
            border-collapse: collapse;
			min-width: 920px;
            overflow: hidden;
            border-radius: 14px;
        }
        th, td {
            padding: 12px 14px;
            border-top: 1px solid var(--line);
            text-align: left;
            font-size: 14px;
            white-space: nowrap;
            vertical-align: middle;
        }
        th {
            background: #eef4ff;
            color: #0f172a;
            border-top: none;
            font-weight: 700;
        }
        tr:nth-child(even) td { background: #fbfdff; }
        .rank { color: var(--accent); font-weight: 700; }
        .copy-btn, .sub-btn {
            appearance: none;
            border: 0;
            border-radius: 999px;
            padding: 9px 14px;
            font-size: 13px;
            font-weight: 700;
            cursor: pointer;
            transition: transform 0.16s ease, box-shadow 0.16s ease, background 0.16s ease;
            text-decoration: none;
            display: inline-flex;
            align-items: center;
            justify-content: center;
            gap: 8px;
        }
		.copy-btn.icon-btn {
			width: 40px;
			height: 40px;
			padding: 0;
			border-radius: 12px;
		}
		.copy-btn.icon-btn svg {
			width: 18px;
			height: 18px;
			fill: currentColor;
			display: block;
		}
        .copy-btn {
            background: var(--accent);
            color: #fff;
            box-shadow: 0 8px 18px rgba(11, 110, 249, 0.18);
        }
        .copy-btn:hover, .sub-btn:hover { transform: translateY(-1px); }
        .copy-btn:disabled {
            cursor: not-allowed;
            background: #94a3b8;
            box-shadow: none;
        }
        .copy-btn.is-copied { background: #16a34a; }
		.action-cell {
			min-width: 120px;
			display: flex;
			gap: 8px;
			align-items: center;
		}
		.action-cell .copy-btn + .copy-btn {
			margin-left: 0;
		}
				.modal {
					display: none;
					position: fixed;
					inset: 0;
					background: rgba(15, 23, 42, 0.58);
					backdrop-filter: blur(8px);
					align-items: center;
					justify-content: center;
					padding: 16px;
					z-index: 50;
				}
				.modal.is-open {
					display: flex;
				}
				.modal-card {
					width: min(360px, 100%);
					background: #fff;
					border-radius: 18px;
					padding: 18px;
					box-shadow: 0 20px 40px rgba(0, 0, 0, 0.18);
				}
				.modal-head {
					display: flex;
					align-items: center;
					justify-content: space-between;
					gap: 12px;
					margin-bottom: 14px;
				}
				.modal-title {
					font-size: 16px;
					font-weight: 700;
				}
				.modal-close {
					border: 0;
					background: #eef4ff;
					color: #0f172a;
					width: 34px;
					height: 34px;
					border-radius: 999px;
					cursor: pointer;
					font-size: 20px;
					line-height: 1;
				}
				.modal-qr {
					display: block;
					width: 240px;
					height: 240px;
					margin: 0 auto;
					border-radius: 14px;
					border: 1px solid #e5e7eb;
					background: #fff;
				}
				.qr-loading {
					display: none;
					width: 240px;
					height: 240px;
					margin: 0 auto;
					border-radius: 14px;
					border: 1px solid #e5e7eb;
					background: linear-gradient(135deg, #f8fafc, #eef4ff);
					position: relative;
				}
				.qr-loading.is-visible {
					display: block;
				}
				.qr-loading::after {
					content: "";
					position: absolute;
					top: 50%;
					left: 50%;
					width: 36px;
					height: 36px;
					margin-top: -18px;
					margin-left: -18px;
					border-radius: 50%;
					border: 3px solid #cfe0ff;
					border-top-color: var(--accent);
					animation: spin 0.8s linear infinite;
				}
				@keyframes spin {
					to { transform: rotate(360deg); }
				}
				.modal-desc {
					margin: 14px 0 0;
					font-size: 13px;
					color: var(--text-soft);
					line-height: 1.6;
					word-break: break-all;
				}
        .footer {
            padding: 8px 24px 24px;
        }
        .footer-note {
            margin: 0 0 12px;
            color: var(--text-soft);
            font-size: 13px;
            line-height: 1.6;
        }
        .footer-actions {
            display: flex;
            flex-wrap: wrap;
            gap: 12px;
            align-items: center;
        }
        .sub-btn {
            background: var(--accent-soft);
            color: var(--accent);
            border: 1px solid #cfe0ff;
        }
        .notice {
            margin: 10px 24px 0;
            padding: 12px 14px;
            border-radius: 12px;
            background: #fff7ed;
            color: #9a3412;
            border: 1px solid #fed7aa;
            font-size: 13px;
            line-height: 1.6;
        }
        .empty-row {
            text-align: center;
            color: var(--text-soft);
            padding: 24px 14px;
        }
        @media (max-width: 900px) {
            .meta { grid-template-columns: 1fr; }
			.action-cell { min-width: 72px; }
        }
    </style>
</head>
<body>
    <div class="wrap">
        <section class="card">
            <div class="hero">
            </div>

			{{.CurrentSection}}
            {{.HistorySection}}

            <div class="footer">
                <p class="footer-note">提示：请先复制对应的 Vmess 订阅链接，再到其他格式订阅转换工具中使用；本地历史数据会保存在 {{.HistoryFile}} 中。</p>
                <div class="footer-actions">
                    <a class="sub-btn" href="https://subconverters.com/" target="_blank" rel="noreferrer">获取其他格式订阅</a>
                </div>
            </div>
        </section>
    </div>
	<div class="modal" id="qr-modal" aria-hidden="true">
		<div class="modal-card" role="dialog" aria-modal="true" aria-labelledby="qr-title">
			<div class="modal-head">
				<div class="modal-title" id="qr-title">订阅二维码</div>
				<button class="modal-close" type="button" onclick="closeQr()" aria-label="关闭">×</button>
			</div>
			<div class="qr-loading" id="qr-loading" aria-hidden="true"></div>
			<img class="modal-qr" id="qr-image" alt="订阅二维码" src="">
			<p class="modal-desc" id="qr-text"></p>
		</div>
	</div>

    <script>
		var qrModal = document.getElementById('qr-modal');
		var qrImage = document.getElementById('qr-image');
		var qrText = document.getElementById('qr-text');

        function copyText(text) {
            if (navigator.clipboard && window.isSecureContext) {
                return navigator.clipboard.writeText(text);
            }
            return new Promise(function(resolve, reject) {
                var textArea = document.createElement('textarea');
                textArea.value = text;
                textArea.style.position = 'fixed';
                textArea.style.opacity = '0';
                document.body.appendChild(textArea);
                textArea.focus();
                textArea.select();
                try {
                    var ok = document.execCommand('copy');
                    if (ok) {
                        resolve();
                    } else {
                        reject(new Error('copy failed'));
                    }
                } catch (error) {
                    reject(error);
                } finally {
                    document.body.removeChild(textArea);
                }
            });
        }

        function copyVmess(button) {
            var link = button.getAttribute('data-vmess');
            if (!link) {
                alert('请先在 result.yaml 中配置 vmess_template');
                return;
            }

			var originalHtml = button.innerHTML;
            copyText(link).then(function() {
				button.innerHTML = '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M20 6L9 17l-5-5 1.41-1.41L9 14.17 18.59 4.59z"/></svg>';
                button.classList.add('is-copied');
                setTimeout(function() {
                    button.innerHTML = originalHtml;
                    button.classList.remove('is-copied');
                }, 1400);
            }).catch(function() {
                alert('复制失败，请手动复制该链接。');
            });
        }
		function showQr(button) {
			var link = button.getAttribute('data-vmess');
			if (!link) {
				alert('请先在 result.yaml 中配置 vmess_template');
				return;
			}
			qrLoading.classList.add('is-visible');
			qrImage.style.display = 'none';
			qrImage.onload = function() {
				qrLoading.classList.remove('is-visible');
				qrImage.style.display = 'block';
			};
			qrImage.onerror = function() {
				qrLoading.classList.remove('is-visible');
				alert('二维码加载失败，请稍后重试。');
			};
			qrImage.src = 'https://api.qrserver.com/v1/create-qr-code/?size=240x240&data=' + encodeURIComponent(link);
			qrText.textContent = link;
			qrModal.classList.add('is-open');
			qrModal.setAttribute('aria-hidden', 'false');
		}

		function closeQr() {
			qrModal.classList.remove('is-open');
			qrModal.setAttribute('aria-hidden', 'true');
			qrLoading.classList.remove('is-visible');
			qrImage.src = '';
			qrImage.style.display = 'block';
			qrText.textContent = '';
		}

		qrModal.addEventListener('click', function(event) {
			if (event.target === qrModal) {
				closeQr();
			}
		});

		document.addEventListener('keydown', function(event) {
			if (event.key === 'Escape') {
				closeQr();
			}
		});

		// 标签页已移除：本次结果和最近历史结果顺序展示。
    </script>
</body>
</html>`))

	page := struct {
		CurrentSection htmpl.HTML
		HistorySection htmpl.HTML
		HistoryFile    string
	}{
		CurrentSection: currentSection,
		HistorySection: historySection,
		HistoryFile:    HistoryOutputPath,
	}

	fp, err := os.Create(HTMLOutputPath)
	if err != nil {
		log.Fatalf("创建文件[%s]失败：%v", HTMLOutputPath, err)
		return
	}
	defer fp.Close()

	if err := pageTemplate.Execute(fp, page); err != nil {
		log.Fatalf("写入文件[%s]失败：%v", HTMLOutputPath, err)
		return
	}

	fmt.Printf("HTML 测速结果已写入 %v 文件，可用浏览器打开查看。\n", HTMLOutputPath)
}
