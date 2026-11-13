package xadmin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// tuiScreen 是交互式界面的当前页面。
type tuiScreen int

const (
	screenMenu tuiScreen = iota
	screenBusy
	screenAPIForm
	screenHandlerForm
	screenInitForm
	screenPlan
	screenConfirm
	screenOutput
)

type tuiMenuItem struct {
	title string
	desc  string
}

var tuiMenu = []tuiMenuItem{
	{title: "升级框架", desc: "检测最新版本，生成计划并执行三方合并"},
	{title: "检查更新", desc: "查询 GitHub 最新 Release，不修改工作区"},
	{title: "创建 API 面", desc: "生成 handler、路由和可选 JWT 认证"},
	{title: "创建 Handler", desc: "在指定 API 面/版本下生成资源路由"},
	{title: "项目体检", desc: "检查版本、配置、迁移和前端工具链"},
	{title: "构建校验", desc: "运行 go build 与 go vet"},
	{title: "回滚升级", desc: "恢复最近一次升级前的文件快照"},
	{title: "初始化基准", desc: "为复制源码/新 git 项目写入框架版本"},
	{title: "版本信息", desc: "查看工具和运行环境版本"},
	{title: "退出", desc: "关闭交互式界面"},
}

const (
	tuiMenuUpgrade = iota
	tuiMenuCheck
	tuiMenuCreateAPI
	tuiMenuCreateHandler
	tuiMenuDoctor
	tuiMenuVerify
	tuiMenuRollback
	tuiMenuInit
	tuiMenuVersion
	tuiMenuQuit
)

var (
	tuiAccent = lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#C4B5FD"}
	tuiMuted  = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"}
	tuiGreen  = lipgloss.AdaptiveColor{Light: "#047857", Dark: "#34D399"}
	tuiRed    = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"}

	tuiTitleStyle    = lipgloss.NewStyle().Bold(true).Foreground(tuiAccent)
	tuiSelectedStyle = lipgloss.NewStyle().Bold(true).Foreground(tuiAccent)
	tuiMutedStyle    = lipgloss.NewStyle().Foreground(tuiMuted)
	tuiGreenStyle    = lipgloss.NewStyle().Foreground(tuiGreen)
	tuiRedStyle      = lipgloss.NewStyle().Foreground(tuiRed)
)

type tuiPlanMsg struct {
	res     *Resolved
	plan    *Plan
	warning string
	err     error
}

type tuiTextMsg struct {
	title   string
	output  string
	err     error
	refresh bool
	form    tuiScreen
}

type tuiModel struct {
	width  int
	height int
	screen tuiScreen

	menuCursor int

	version string
	branch  string
	clean   bool
	isGit   bool

	busy      bool
	busyTitle string
	status    string
	statusErr bool
	spinner   spinner.Model

	inputs      []textinput.Model
	fieldLabels []string
	fieldHints  []string
	formKind    string
	formFocus   int
	auth        bool

	viewport    viewport.Model
	outputTitle string

	resolved *Resolved
	plan     *Plan
	warning  string

	confirmKind   string
	confirmTitle  string
	confirmBody   string
	confirmReturn tuiScreen
}

func newTUIModel() *tuiModel {
	m := &tuiModel{
		screen:   screenMenu,
		spinner:  spinner.New(spinner.WithSpinner(spinner.Dot)),
		viewport: viewport.New(80, 20),
	}
	m.refreshStatus()
	return m
}

func (m *tuiModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m *tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.busy {
			return m, cmd
		}
		return m, nil
	case tuiPlanMsg:
		m.showPlan(msg)
		return m, nil
	case tuiTextMsg:
		m.showText(msg)
		return m, nil
	}
	return m, nil
}

func (m *tuiModel) View() string {
	switch m.screen {
	case screenMenu:
		return m.viewMenu()
	case screenBusy:
		return m.viewBusy()
	case screenAPIForm, screenHandlerForm, screenInitForm:
		return m.viewForm()
	case screenPlan:
		return m.viewPlan()
	case screenConfirm:
		return m.viewConfirm()
	case screenOutput:
		return m.viewOutput()
	default:
		return m.viewMenu()
	}
}

func (m *tuiModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if m.busy {
		return m, nil
	}
	switch m.screen {
	case screenMenu:
		return m.updateMenu(msg)
	case screenAPIForm, screenHandlerForm, screenInitForm:
		return m.updateForm(msg)
	case screenPlan:
		return m.updatePlan(msg)
	case screenConfirm:
		return m.updateConfirm(msg)
	case screenOutput:
		return m.updateOutput(msg)
	}
	return m, nil
}

func (m *tuiModel) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
	case "down", "j":
		if m.menuCursor < len(tuiMenu)-1 {
			m.menuCursor++
		}
	case "enter":
		return m.activateMenu()
	case "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m *tuiModel) activateMenu() (tea.Model, tea.Cmd) {
	switch m.menuCursor {
	case tuiMenuUpgrade:
		return m, tea.Batch(m.startBusy("升级框架", "正在检查最新版本并生成升级计划…"), m.cmdPlan())
	case tuiMenuCheck:
		return m, tea.Batch(m.startBusy("检查更新", "正在查询 GitHub 最新 Release…"), m.cmdCheck())
	case tuiMenuCreateAPI:
		m.openAPIForm()
		return m, textinput.Blink
	case tuiMenuCreateHandler:
		m.openHandlerForm()
		return m, textinput.Blink
	case tuiMenuDoctor:
		return m, tea.Batch(m.startBusy("项目体检", "正在检查版本、配置、迁移和工具链…"), m.cmdDoctor())
	case tuiMenuVerify:
		return m, tea.Batch(m.startBusy("构建校验", "正在运行 go build 与 go vet…"), m.cmdVerify())
	case tuiMenuRollback:
		m.confirmKind = "rollback"
		m.confirmTitle = "回滚最近一次升级？"
		m.confirmBody = "将恢复最近一次升级前备份的文件。\n未提交的改动可能被覆盖，请先确认 git status。"
		m.confirmReturn = screenMenu
		m.screen = screenConfirm
		return m, nil
	case tuiMenuInit:
		m.openInitForm()
		return m, textinput.Blink
	case tuiMenuVersion:
		var b strings.Builder
		printVersionTo(&b)
		m.showText(tuiTextMsg{title: "版本信息", output: b.String()})
		return m, nil
	case tuiMenuQuit:
		return m, tea.Quit
	}
	return m, nil
}

func (m *tuiModel) openAPIForm() {
	m.formKind = "api"
	m.fieldLabels = []string{"API 面名称 *", "版本", "路径前缀", "认证用户表", "迁移编号"}
	m.fieldHints = []string{"client", "v1", "/client/v1", "client_users", "自动 max+1"}
	m.setInputs([]string{"", "v1", "", "", ""})
	m.auth = false
	m.formFocus = 0
	m.screen = screenAPIForm
}

func (m *tuiModel) openHandlerForm() {
	m.formKind = "handler"
	m.fieldLabels = []string{"API 面 *", "版本", "资源名 *", "路由路径", "权限码"}
	m.fieldHints = []string{"admin / client", "v1", "order", "/orders", "system:order"}
	m.setInputs([]string{"admin", "v1", "", "", ""})
	m.formFocus = 0
	m.screen = screenHandlerForm
}

func (m *tuiModel) openInitForm() {
	m.formKind = "init"
	m.fieldLabels = []string{"当前框架版本 *", "上游仓库"}
	m.fieldHints = []string{"v1.3.0", DefaultRepo}
	m.setInputs([]string{"", DefaultRepo})
	m.formFocus = 0
	m.screen = screenInitForm
}

func (m *tuiModel) setInputs(values []string) {
	m.inputs = make([]textinput.Model, len(values))
	for i, value := range values {
		ti := textinput.New()
		ti.Prompt = ""
		ti.Placeholder = m.fieldHints[i]
		ti.SetValue(value)
		ti.CharLimit = 128
		ti.Width = 48
		m.inputs[i] = ti
	}
	m.focusField(0)
}

func (m *tuiModel) focusField(index int) tea.Cmd {
	if len(m.inputs) == 0 {
		return nil
	}
	if index < 0 {
		index = len(m.inputs) - 1
	}
	index %= len(m.inputs)
	var cmd tea.Cmd
	for i := range m.inputs {
		if i == index {
			cmd = m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
	m.formFocus = index
	return cmd
}

func (m *tuiModel) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		m.status = ""
		return m, nil
	case "tab", "down":
		return m, m.focusField(m.formFocus + 1)
	case "shift+tab", "up":
		return m, m.focusField(m.formFocus - 1)
	case "ctrl+t":
		if m.formKind == "api" {
			m.auth = !m.auth
		}
		return m, nil
	case "enter":
		return m.submitForm()
	}
	var cmd tea.Cmd
	m.inputs[m.formFocus], cmd = m.inputs[m.formFocus].Update(msg)
	return m, cmd
}

func (m *tuiModel) submitForm() (tea.Model, tea.Cmd) {
	value := func(i int) string {
		if i >= len(m.inputs) {
			return ""
		}
		return strings.TrimSpace(m.inputs[i].Value())
	}
	switch m.formKind {
	case "api":
		name := value(0)
		if name == "" {
			m.status = "API 面名称不能为空"
			m.statusErr = true
			return m, nil
		}
		opts := CreateAPIOptions{
			Dir:     tuiProjectDir(),
			Name:    name,
			Version: value(1),
			Prefix:  value(2),
			Auth:    m.auth,
		}
		if m.auth {
			opts.Table = value(3)
			opts.Migration = value(4)
		}
		status := "正在生成 handler 与路由…"
		if m.auth {
			status = "正在生成 handler、路由和认证迁移…"
		}
		tick := m.startBusy("创建 API 面", status)
		return m, tea.Batch(tick, m.cmdCreateAPI(opts))
	case "handler":
		api, name := value(0), value(2)
		if api == "" || name == "" {
			m.status = "API 面名称和资源名不能为空"
			m.statusErr = true
			return m, nil
		}
		opts := CreateHandlerOptions{
			Dir:        tuiProjectDir(),
			API:        api,
			Version:    value(1),
			Name:       name,
			Path:       value(3),
			Permission: value(4),
		}
		return m, tea.Batch(m.startBusy("创建 Handler", "正在生成 handler 与路由模块…"), m.cmdCreateHandler(opts))
	case "init":
		from := value(0)
		if from == "" {
			m.status = "当前框架版本不能为空，例如 v1.3.0"
			m.statusErr = true
			return m, nil
		}
		return m, tea.Batch(m.startBusy("初始化基准", "正在解析上游版本并写入 FRAMEWORK_VERSION…"), m.cmdInit(value(1), from))
	}
	return m, nil
}

func (m *tuiModel) updatePlan(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.screen = screenMenu
		m.status = ""
		return m, nil
	case "enter":
		if m.plan == nil || !m.plan.HasWork() {
			return m, nil
		}
		if m.plan.HasMigrationErrors() {
			m.status = "存在迁移编号冲突，请先按 docs/tool/upgrade.md 处理"
			m.statusErr = true
			return m, nil
		}
		if !m.worktreeClean() {
			m.status = "工作区有未提交改动；请先提交/暂存，或使用命令行 --allow-dirty"
			m.statusErr = true
			return m, nil
		}
		m.openUpgradeConfirm()
		return m, nil
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *tuiModel) openUpgradeConfirm() {
	s := m.plan.ComputeSummary()
	body := fmt.Sprintf("%s → %s\n\nupdate %d · add %d · delete %d · keep %d · conflict %d\n\n将创建升级分支，并运行 go mod tidy（按需）、go build、go vet。",
		labelWithCommit(m.plan.FromRef, m.plan.FromCommit),
		labelWithCommit(m.plan.ToRef, m.plan.ToCommit),
		s.Update, s.Add, s.Delete, s.Keep, s.Conflict)
	if s.Conflict > 0 {
		body += "\n\n仍有文件冲突，升级会保留本地并写入冲突标记或 .upstream 文件。"
	}
	m.confirmKind = "upgrade"
	m.confirmTitle = "执行升级？"
	m.confirmBody = body
	m.confirmReturn = screenPlan
	m.screen = screenConfirm
}

func (m *tuiModel) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "n":
		m.screen = m.confirmReturn
		return m, nil
	case "enter", "y":
		switch m.confirmKind {
		case "upgrade":
			if m.resolved == nil || m.plan == nil {
				m.status = "升级计划尚未生成"
				m.statusErr = true
				m.screen = screenMenu
				return m, nil
			}
			return m, tea.Batch(m.startBusy("升级框架", "正在执行三方合并与构建校验…"), m.cmdApply())
		case "rollback":
			return m, tea.Batch(m.startBusy("回滚升级", "正在恢复最近一次升级前的文件快照…"), m.cmdRollback())
		}
	}
	return m, nil
}

func (m *tuiModel) updateOutput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.screen = screenMenu
		m.status = ""
		return m, nil
	}
	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m *tuiModel) startBusy(title, status string) tea.Cmd {
	m.busy = true
	m.busyTitle = title
	m.status = status
	m.statusErr = false
	m.screen = screenBusy
	return m.spinner.Tick
}

func (m *tuiModel) showPlan(msg tuiPlanMsg) {
	m.busy = false
	m.status = ""
	if msg.err != nil || msg.plan == nil {
		err := msg.err
		if err == nil {
			err = fmt.Errorf("升级计划为空")
		}
		m.showText(tuiTextMsg{title: "升级计划", err: err})
		return
	}
	m.resolved = msg.res
	m.plan = msg.plan
	m.warning = msg.warning
	m.screen = screenPlan
	var b strings.Builder
	if msg.warning != "" {
		b.WriteString(tuiMutedStyle.Render("⚠ "+msg.warning) + "\n\n")
	}
	PrintPlan(&b, msg.plan, true, false)
	m.viewport.SetContent(strings.TrimRight(b.String(), "\n"))
	m.viewport.GotoTop()
}

func (m *tuiModel) showText(msg tuiTextMsg) {
	m.busy = false
	m.status = ""
	if msg.refresh {
		m.refreshStatus()
	}
	if msg.err != nil && msg.form != screenMenu {
		m.screen = msg.form
		m.status = msg.err.Error()
		m.statusErr = true
		return
	}
	m.screen = screenOutput
	m.outputTitle = msg.title
	var b strings.Builder
	if msg.output != "" {
		b.WriteString(strings.TrimRight(msg.output, "\n"))
		b.WriteString("\n")
	}
	if msg.err != nil {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(tuiRedStyle.Render("✖ " + msg.err.Error()))
		b.WriteString("\n")
	}
	if b.Len() == 0 {
		b.WriteString("（无输出）\n")
	}
	m.viewport.SetContent(strings.TrimRight(b.String(), "\n"))
	m.viewport.GotoTop()
}

func (m *tuiModel) refreshStatus() {
	m.version = readFrameworkVersion()
	git, err := NewGit(".")
	if err != nil {
		m.isGit = false
		m.branch = ""
		m.clean = false
		return
	}
	m.isGit = true
	m.branch = git.Branch()
	clean, _, err := git.IsClean()
	m.clean = err == nil && clean
}

func readFrameworkVersion() string {
	data, err := os.ReadFile(filepath.Join(tuiProjectDir(), VersionPath))
	if err != nil {
		return "未设置"
	}
	version := strings.TrimSpace(string(data))
	if version == "" {
		return "未设置"
	}
	return version
}

// tuiProjectDir 优先返回 git 仓库根目录，保证从子目录启动时也能找到项目文件。
func tuiProjectDir() string {
	if git, err := NewGit("."); err == nil && git.Dir != "" {
		return git.Dir
	}
	return "."
}

// worktreeClean 在真正执行升级前再检查一次，避免计划页停留期间工作区发生变化。
func (m *tuiModel) worktreeClean() bool {
	git, err := NewGit(".")
	if err != nil {
		return true // 交给 ExecuteUpgrade 报更准确的错误
	}
	clean, _, err := git.IsClean()
	if err != nil {
		return true
	}
	return clean
}

func (m *tuiModel) resize() {
	width := m.width
	if width <= 0 {
		width = 80
	}
	height := m.height
	if height <= 0 {
		height = 24
	}
	vpWidth := width - 2
	if vpWidth < 40 {
		vpWidth = 40
	}
	vpHeight := height - 8
	if vpHeight < 6 {
		vpHeight = 6
	}
	m.viewport.Width = vpWidth
	m.viewport.Height = vpHeight
	inputWidth := width - 34
	if inputWidth < 24 {
		inputWidth = 24
	}
	for i := range m.inputs {
		m.inputs[i].Width = inputWidth
	}
}

func (m *tuiModel) header() string {
	title := tuiTitleStyle.Render("x-admin 工具链") + "  " + tuiMutedStyle.Render("v"+ToolVersion)
	var status []string
	status = append(status, "框架 "+m.version)
	if m.isGit {
		status = append(status, m.branch)
		if m.clean {
			status = append(status, tuiGreenStyle.Render("工作区干净"))
		} else {
			status = append(status, tuiRedStyle.Render("有未提交改动"))
		}
	} else {
		status = append(status, tuiRedStyle.Render("非 git 仓库"))
	}
	width := m.width
	if width <= 0 {
		width = 80
	}
	rule := tuiMutedStyle.Render(strings.Repeat("─", max(20, width-1)))
	return title + "\n" + tuiMutedStyle.Render(strings.Join(status, " · ")) + "\n" + rule + "\n\n"
}

func (m *tuiModel) footer(hint string) string {
	var parts []string
	if m.busy {
		parts = append(parts, m.spinner.View()+" "+m.status)
	} else if m.status != "" {
		if m.statusErr {
			parts = append(parts, tuiRedStyle.Render("✖ "+m.status))
		} else {
			parts = append(parts, tuiMutedStyle.Render(m.status))
		}
	}
	if hint != "" {
		parts = append(parts, tuiMutedStyle.Render(hint))
	}
	return strings.Join(parts, "\n")
}

func (m *tuiModel) viewMenu() string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString(tuiTitleStyle.Render("主菜单"))
	b.WriteString("\n\n")
	for i, item := range tuiMenu {
		selected := i == m.menuCursor
		cursor := "  "
		title := item.title
		if selected {
			cursor = "› "
			title = tuiSelectedStyle.Render(title)
		}
		line := "  " + cursor + title
		if m.width == 0 || m.width >= 76 {
			line += "  " + tuiMutedStyle.Render(item.desc)
		}
		b.WriteString(line + "\n")
		if i == 1 || i == 3 || i == 6 || i == 8 {
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(m.footer("↑/↓ 选择 · enter 确认 · q 退出"))
	return b.String()
}

func (m *tuiModel) viewBusy() string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString(tuiTitleStyle.Render(m.busyTitle))
	b.WriteString("\n\n  ")
	b.WriteString(m.spinner.View())
	b.WriteString(" ")
	b.WriteString(m.status)
	b.WriteString("\n\n")
	b.WriteString(tuiMutedStyle.Render("操作完成后会自动显示结果 · ctrl+c 退出"))
	return b.String()
}

func (m *tuiModel) viewForm() string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString(tuiTitleStyle.Render(m.formTitle()))
	b.WriteString("\n\n")
	for i := range m.inputs {
		marker := "  "
		if i == m.formFocus {
			marker = "› "
		}
		b.WriteString("  " + marker + padRight(m.fieldLabels[i], 18) + m.inputs[i].View() + "\n")
	}
	if m.formKind == "api" {
		check := "[ ]"
		if m.auth {
			check = "[x]"
		}
		b.WriteString("\n  " + check + " 生成 JWT 认证（注册/登录/刷新/登出/me） · ctrl+t 切换\n")
	}
	b.WriteString("\n")
	b.WriteString(m.footer("tab 切换 · enter 提交 · esc 返回"))
	return b.String()
}

func (m *tuiModel) formTitle() string {
	switch m.formKind {
	case "api":
		return "创建 API 面"
	case "handler":
		return "创建 Handler"
	case "init":
		return "初始化框架基准"
	default:
		return "表单"
	}
}

func (m *tuiModel) viewPlan() string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString(tuiTitleStyle.Render("升级计划"))
	b.WriteString("\n\n")
	b.WriteString(m.viewport.View())
	b.WriteString("\n")
	hint := "esc 返回"
	if m.plan != nil && m.plan.HasWork() {
		if m.plan.HasMigrationErrors() {
			hint = "存在迁移编号冲突 · esc 返回处理"
		} else if m.plan.HasConflicts() {
			hint = "enter 执行升级（有冲突） · esc 返回 · ↑/↓ 滚动"
		} else {
			hint = "enter 执行升级 · esc 返回 · ↑/↓ 滚动"
		}
	}
	if m.plan != nil && !m.plan.HasWork() {
		hint = "已是最新 · esc 返回"
	}
	b.WriteString(m.footer(hint))
	return b.String()
}

func (m *tuiModel) viewConfirm() string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString(tuiTitleStyle.Render(m.confirmTitle))
	b.WriteString("\n\n")
	b.WriteString(m.confirmBody)
	b.WriteString("\n\n")
	b.WriteString(m.footer("enter 确认 · esc 取消"))
	return b.String()
}

func (m *tuiModel) viewOutput() string {
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteString(tuiTitleStyle.Render(m.outputTitle))
	b.WriteString("\n\n")
	b.WriteString(m.viewport.View())
	b.WriteString("\n")
	b.WriteString(m.footer("↑/↓ 滚动 · ←/→ 横向 · esc 返回"))
	return b.String()
}

func padRight(value string, width int) string {
	current := lipgloss.Width(value)
	if current >= width {
		return value
	}
	return value + strings.Repeat(" ", width-current)
}

// shouldUseTUI 只有在真正的交互式终端里才自动打开 TUI。
// 管道、CI、以及 XADMIN_NO_TUI=1 都回退到文本帮助。
func shouldUseTUI() bool {
	if strings.TrimSpace(os.Getenv("XADMIN_NO_TUI")) != "" {
		return false
	}
	if strings.TrimSpace(os.Getenv("CI")) != "" {
		return false
	}
	return isTerminal(os.Stdin) && isTerminal(os.Stdout)
}

func runTUI() int {
	program := tea.NewProgram(newTUIModel(), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI 启动失败: %v\n", err)
		return 1
	}
	return 0
}

func (m *tuiModel) cmdCheck() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		result, err := CheckUpgrade(ctx, SourceOptions{
			Repo: DefaultRepo, Dir: tuiProjectDir(), Timeout: 60 * time.Second,
		})
		if err != nil {
			return tuiTextMsg{title: "检查更新", err: err}
		}
		var b strings.Builder
		PrintCheck(&b, result, true)
		return tuiTextMsg{title: "检查更新", output: b.String()}
	}
}

func (m *tuiModel) cmdPlan() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		res, plan, warning, err := PrepareUpgrade(ctx, SourceOptions{
			Repo: DefaultRepo, Dir: tuiProjectDir(), Timeout: 10 * time.Minute,
		})
		return tuiPlanMsg{res: res, plan: plan, warning: warning, err: err}
	}
}

func (m *tuiModel) cmdApply() tea.Cmd {
	res, plan := m.resolved, m.plan
	return func() tea.Msg {
		var b strings.Builder
		err := ExecuteUpgrade(context.Background(), res, plan, ApplyOptions{Yes: true}, &b)
		return tuiTextMsg{title: "升级框架", output: b.String(), err: err, refresh: true}
	}
}

func (m *tuiModel) cmdCreateAPI(opts CreateAPIOptions) tea.Cmd {
	return func() tea.Msg {
		result, err := CreateAPI(opts)
		if err != nil {
			return tuiTextMsg{title: "创建 API 面", err: err, form: screenAPIForm}
		}
		var b strings.Builder
		printCreateResultTo(&b, result)
		return tuiTextMsg{title: "创建 API 面", output: b.String(), refresh: true}
	}
}

func (m *tuiModel) cmdCreateHandler(opts CreateHandlerOptions) tea.Cmd {
	return func() tea.Msg {
		result, err := CreateHandler(opts)
		if err != nil {
			return tuiTextMsg{title: "创建 Handler", err: err, form: screenHandlerForm}
		}
		var b strings.Builder
		printCreateResultTo(&b, result)
		return tuiTextMsg{title: "创建 Handler", output: b.String(), refresh: true}
	}
}

func (m *tuiModel) cmdInit(repo, from string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		state, _, err := InitBaseline(ctx, repo, "", from, tuiProjectDir())
		if err != nil {
			return tuiTextMsg{title: "初始化基准", err: err, form: screenInitForm}
		}
		var b strings.Builder
		fmt.Fprintf(&b, "✔ 已记录框架基准：%s (%s)\n", state.Ref, ShortSHA(state.Commit))
		b.WriteString("请提交 FRAMEWORK_VERSION；之后升级会以它为基准。\n")
		return tuiTextMsg{title: "初始化基准", output: b.String(), refresh: true}
	}
}

func (m *tuiModel) cmdDoctor() tea.Cmd {
	return func() tea.Msg {
		checks, err := RunDoctor(context.Background(), DoctorOptions{Dir: tuiProjectDir(), Repo: DefaultRepo})
		if err != nil {
			return tuiTextMsg{title: "项目体检", err: err}
		}
		var b strings.Builder
		PrintDoctor(&b, checks)
		return tuiTextMsg{title: "项目体检", output: b.String()}
	}
}

func (m *tuiModel) cmdVerify() tea.Cmd {
	return func() tea.Msg {
		var b strings.Builder
		err := VerifyProject(context.Background(), tuiProjectDir(), VerifyOptions{}, &b)
		return tuiTextMsg{title: "构建校验", output: b.String(), err: err}
	}
}

func (m *tuiModel) cmdRollback() tea.Cmd {
	return func() tea.Msg {
		git, err := NewGit(".")
		if err != nil {
			return tuiTextMsg{title: "回滚升级", err: err}
		}
		backupDir, err := LatestBackup(git)
		if err != nil {
			return tuiTextMsg{title: "回滚升级", err: err}
		}
		n, manifest, err := Rollback(git, git.Dir, backupDir)
		if err != nil {
			return tuiTextMsg{title: "回滚升级", err: err}
		}
		var b strings.Builder
		fmt.Fprintf(&b, "✔ 已恢复 %d 个文件（备份：%s）\n", n, backupDir)
		if manifest.Branch != "" {
			fmt.Fprintf(&b, "升级分支：%s", manifest.Branch)
			if manifest.OriginalBranch != "" {
				fmt.Fprintf(&b, "，原分支：%s", manifest.OriginalBranch)
			}
			b.WriteString("\n")
		}
		b.WriteString("请运行 git status 检查工作区。\n")
		return tuiTextMsg{title: "回滚升级", output: b.String(), refresh: true}
	}
}
