import React, { useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react'
import { createRoot } from 'react-dom/client'
import { ArrowDownToLine, ArrowLeft, ArrowRight, BookOpen, CalendarDays, Check, ChevronDown, ChevronLeft, ChevronRight, Compass, Grid2X2, Heart, ListFilter, Search, Settings2, Sparkles, X } from 'lucide-react'
import { Badge, Button, Field, Row, Stage, Tabs, Window } from './kit'
import Gold from './gold'
import './study.css'

type Direction = 'shelf' | 'daily' | 'cinema'
type Anime = { id: string; title: string; day: number; cover: string }
const DAYS = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
const OPTIONS: { id: Direction; number: string; name: string; line: string; description: string; flow: string; icon: typeof Grid2X2 }[] = [
  { id: 'shelf', number: '01', name: '海报书架', line: '慢慢逛，遇到想看的。', description: '让海报成为第一眼。筛选退到边上，点开侧栏了解作品，关上后接着逛。', flow: '扫海报 → 侧栏预览 → 选择版本', icon: Grid2X2 },
  { id: 'daily', number: '02', name: '每日放送', line: '这一天，有什么可看？', description: '用星期组织这一季。左边快速扫片，右边直接看详情，适合每天回来找更新。', flow: '选星期 → 切换作品 → 就地追番', icon: CalendarDays },
  { id: 'cinema', number: '03', name: '沉浸选片', line: '先被一部作品打动。', description: '一次只聚焦一部。大幅画面与下方片单联动，少一点管理感，多一点选片的期待。', flow: '翻片单 → 聚焦作品 → 弹层选版', icon: Sparkles },
]
const VERSION_OPTIONS = [
  { name: '版本 A', note: '简体中文 · 1080P · MKV', extra: '适合大多数设备' },
  { name: '版本 B', note: '简繁中文 · 1080P · MP4', extra: '偏好 MP4 容器' },
]
const PINNED = ['4063', '4031', '3887', '4059', '4056', '4070', '4105', '4072', '4086', '4062', '4082', '4118', '4085', '4057', '4093', '4087', '3984', '4058', '4068']

function Action({ children, onClick, variant = 'secondary', className = '', icon, target, label, disabled = false }: { children: ReactNode; onClick: () => void; variant?: 'primary' | 'secondary' | 'ghost' | 'tertiary'; className?: string; icon?: ReactNode; target?: string; label?: string; disabled?: boolean }) {
  return <Button variant={variant} size="large" className={className} icon={icon} onClick={() => { if (!disabled) onClick() }} tabIndex={0} aria-label={label} aria-disabled={disabled || undefined} data-target={target} onKeyDown={(event) => {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      if (!disabled) onClick()
    }
  }}>{children}</Button>
}

function SearchBox({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const input = useRef<HTMLInputElement>(null)
  return <div className="search-box">
    <Field value={value} placeholder="搜索番剧" leading={<Search size={15} />} />
    <input ref={input} value={value} onChange={(event) => onChange(event.target.value)} placeholder="搜索番剧" aria-label="搜索番剧" />
    {value && <button className="search-clear" onClick={() => { onChange(''); input.current?.focus() }} aria-label="清空搜索"><X size={13} /></button>}
  </div>
}

function Poster({ anime, className = '' }: { anime: Anime; className?: string }) {
  return <img className={`poster ${className}`} src={`/art/${anime.cover}`} alt={anime.title} draggable={false} />
}

function Sidebar({ direction, count, onOther }: { direction: Direction; count: number; onOther: () => void }) {
  return <aside className="app-sidebar">
    <div className="sidebar-brand"><img src="/art/dipper.svg" alt="" /><strong>Dipper</strong></div>
    <div className="sidebar-caption">你的动画时光</div>
    <nav aria-label="应用导航">
      <div className="nav-item current"><Compass size={18} /><span>浏览番剧</span></div>
      <button className="nav-item" onClick={onOther}><BookOpen size={18} /><span>我的追番</span><small>{count}</small></button>
      <button className="nav-item" onClick={onOther}><ArrowDownToLine size={18} /><span>下载</span></button>
    </nav>
    <div className="sidebar-bottom"><span className="quiet-dot" />本地片单，慢慢看</div>
    <button className="nav-item settings" onClick={onOther}><Settings2 size={17} /><span>设置</span></button>
    {direction === 'cinema' && <span className="cinema-sidebar-word" aria-hidden="true">D</span>}
  </aside>
}

function AppPreview({ direction, catalog, capture = false }: { direction: Direction; catalog: Anime[]; capture?: boolean }) {
  const [day, setDay] = useState(direction === 'daily' ? 2 : -1)
  const [query, setQuery] = useState('')
  const [selected, setSelected] = useState(catalog[0])
  const [detail, setDetail] = useState(false)
  const [versionOpen, setVersionOpen] = useState(false)
  const [version, setVersion] = useState<number | null>(null)
  const [follows, setFollows] = useState<Record<string, number>>({ '4056': 0, '4072': 1 })
  const [notice, setNotice] = useState('')
  const [undoId, setUndoId] = useState<string | null>(null)
  const [onlyUnfollowed, setOnlyUnfollowed] = useState(false)
  const [seasonOpen, setSeasonOpen] = useState(false)
  const dialog = useRef<HTMLElement>(null)
  const detailOpener = useRef<HTMLElement | null>(null)
  const versionOpener = useRef<HTMLElement | null>(null)
  const noticeTimer = useRef<ReturnType<typeof setTimeout>>()
  const filtered = catalog.filter((anime) => (day < 0 || anime.day === day) && anime.title.toLowerCase().includes(query.trim().toLowerCase()) && (!onlyUnfollowed || follows[anime.id] === undefined))
  const isFollowed = follows[selected.id] !== undefined
  const visibleSelected = filtered.find((anime) => anime.id === selected.id) ?? filtered[0] ?? selected
  const current = direction === 'shelf' ? selected : visibleSelected
  const currentFollowed = follows[current.id] !== undefined
  const count = Object.keys(follows).length

  useEffect(() => {
    if (direction === 'daily') setSelected(catalog.find((anime) => anime.day === 2) ?? catalog[0])
    return () => clearTimeout(noticeTimer.current)
  }, [])

  useEffect(() => {
    if (!detail) return
    detailOpener.current = document.activeElement as HTMLElement
    const frame = requestAnimationFrame(() => dialog.current?.querySelector<HTMLElement>('[role="button"], button')?.focus())
    return () => { cancelAnimationFrame(frame); detailOpener.current?.focus() }
  }, [detail])

  useEffect(() => {
    if (!versionOpen) return
    versionOpener.current = document.activeElement as HTMLElement
    const frame = requestAnimationFrame(() => dialog.current?.querySelector<HTMLElement>('[role="button"], button')?.focus())
    return () => { cancelAnimationFrame(frame); versionOpener.current?.focus() }
  }, [versionOpen])

  useEffect(() => {
    if (!detail && !versionOpen) return
    const dismiss = (event: globalThis.KeyboardEvent) => {
      if (event.key !== 'Escape') return
      if (versionOpen) setVersionOpen(false)
      else setDetail(false)
    }
    document.addEventListener('keydown', dismiss)
    return () => document.removeEventListener('keydown', dismiss)
  }, [detail, versionOpen])

  const inform = (message: string, undo: string | null = null) => {
    clearTimeout(noticeTimer.current)
    setNotice(message)
    setUndoId(undo)
    noticeTimer.current = setTimeout(() => { setNotice(''); setUndoId(null) }, 6500)
  }
  const choose = (anime: Anime) => {
    setSelected(anime)
    setVersionOpen(false)
    setVersion(null)
    if (direction === 'shelf') setDetail(true)
  }
  const follow = () => {
    if (version === null) return
    setFollows((previous) => ({ ...previous, [current.id]: version }))
    setVersionOpen(false)
    inform(`已将《${current.title}》加入演示片单`, current.id)
  }
  const undo = () => {
    if (!undoId) return
    setFollows((previous) => { const next = { ...previous }; delete next[undoId]; return next })
    setUndoId(null)
    setNotice('已撤销，本地应用中的追番未受影响')
  }
  const startVersion = () => {
    setSelected(current)
    setVersion(follows[current.id] ?? null)
    setVersionOpen(true)
  }
  const close = () => {
    if (versionOpen) { setVersionOpen(false); return }
    setDetail(false)
  }
  const trap = (event: KeyboardEvent) => {
    if (event.key === 'Escape') { event.stopPropagation(); close() }
    if (event.key !== 'Tab' || !dialog.current) return
    const elements = Array.from(dialog.current.querySelectorAll<HTMLElement>('button, [tabindex="0"], input'))
    const first = elements[0]
    const last = elements[elements.length - 1]
    if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus() }
    else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus() }
  }

  const days = (short = false) => <div className={`weekday-tabs ${short ? 'short-days' : ''}`} role="tablist" aria-label="按星期筛选">
    {[-1, 1, 2, 3, 4, 5, 6, 0].map((value) => <button key={value} role="tab" aria-selected={day === value} className={day === value ? 'active' : ''} onClick={() => { setDay(value); setDetail(false) }} data-target={`day-${value}`}>
      {value === -1 ? '全部' : DAYS[value]}{direction === 'daily' && value !== -1 && <small>{catalog.filter((anime) => anime.day === value).length}</small>}
    </button>)}
  </div>

  const season = <div className="season-wrap">
    <button className="season-button" onClick={() => setSeasonOpen(!seasonOpen)} aria-expanded={seasonOpen}>2026 秋季<ChevronDown size={14} /></button>
    {seasonOpen && <div className="season-popover"><strong>季度示意</strong><p>本次使用本地海报样本。</p><p>先体验布局，季度切换暂未接入。</p><Action variant="ghost" onClick={() => setSeasonOpen(false)}>知道了</Action></div>}
  </div>

  const versionPicker = <div className="version-picker">
    <span className="eyebrow">选择版本</span>
    <h3>以后，就追这个版本。</h3>
    <p>字幕与画质只需选一次。以下为演示选项。</p>
    <div role="radiogroup" aria-label="选择追番版本">
      {VERSION_OPTIONS.map((option, index) => <button className={`version-option ${version === index ? 'chosen' : ''}`} role="radio" aria-checked={version === index} key={option.name} onClick={() => setVersion(index)} data-target={`version-${index}`}>
        <span className="radio-circle">{version === index && <span />}</span>
        <span><strong>{option.name}</strong><span>{option.note}</span><small>{option.extra}</small></span>
        {version === index && <Check size={16} />}
      </button>)}
    </div>
    <Action variant="primary" className={`confirm-follow ${version === null ? 'pending-choice' : ''}`} onClick={follow} disabled={version === null} target="confirm-follow" label="确认追番" icon={<Heart size={16} />}>{version === null ? '先选一个版本' : '确认追番'}</Action>
    <small className="simulation-note">交互演示，不会创建真实订阅或下载</small>
  </div>

  const details = (anime: Anime, compact = false) => <div className={`anime-details ${compact ? 'compact-detail' : ''}`}>
    <span className="eyebrow">{DAYS[anime.day]}放送 · 秋季片单</span>
    <h2>{anime.title}</h2>
    <div className="detail-tags"><Badge>{DAYS[anime.day]}</Badge><Badge>电视动画</Badge>{follows[anime.id] !== undefined && <Badge tone="success" icon={<Check size={11} />}>已追番</Badge>}</div>
    <p className="detail-copy">把想看的留在片单里。选好字幕版本，之后回到「我的追番」，就能接着看。</p>
    <div className="detail-facts"><span>收集方式<strong>跟随所选版本</strong></span><span>片单归属<strong>我的追番</strong></span></div>
    <Action variant="primary" onClick={startVersion} icon={follows[anime.id] !== undefined ? <Check size={16} /> : <Heart size={16} />} target="start-follow">{follows[anime.id] !== undefined ? '查看所选版本' : '选择版本追番'}</Action>
    <p className="helper-copy">{follows[anime.id] !== undefined ? '已留在片单里，前往「我的追番」接着看。' : '先选版本，再开始追番。'}</p>
  </div>

  const noResults = <div className="empty-state"><Search size={27} /><h3>{query ? '没找到这部番' : '这一天，暂时没有样本'}</h3><p>{query ? '换个关键词，或者看看这一季的其他作品。' : '试试周二或周三，继续体验排期浏览。'}</p><Action onClick={() => { setQuery(''); setDay(-1); setOnlyUnfollowed(false) }} variant="secondary">查看全部番剧</Action></div>

  const product = <Window className={`study-window ${direction}`} title="Dipper" toolbar={<div className="windows-actions" aria-hidden="true"><span>—</span><span>□</span><span>×</span></div>} body={{ className: 'product-body' }}>
    <Sidebar direction={direction} count={count} onOther={() => inform('这里先专注「浏览番剧」，其他页面暂未接入')} />
    <main className="browse-main" aria-label={`${OPTIONS.find((option) => option.id === direction)?.name}方向`}>
      <div className="product-top"><span><Compass size={14} /> 浏览番剧</span><div className="top-season">{season}</div><span className="sample-label">本地海报样本</span></div>
      {direction === 'shelf' && <div className="shelf-content">
        <div className="page-heading"><div><span className="eyebrow">2026 · AUTUMN COLLECTION</span><h1>这一季，想看什么？</h1><p>从一张海报开始，找到下一部期待。</p></div><SearchBox value={query} onChange={setQuery} /></div>
        <div className="shelf-filter">{days()}<button className={`unfollowed-filter ${onlyUnfollowed ? 'active' : ''}`} aria-pressed={onlyUnfollowed} onClick={() => setOnlyUnfollowed(!onlyUnfollowed)}><ListFilter size={14} />未追番</button></div>
        <div className="catalog-subline"><span>{query ? `“${query}”的搜索结果` : day < 0 ? '秋季番剧' : `${DAYS[day]}片单`}<small>{filtered.length} 部作品</small></span><span>点开海报，先了解一下 <ArrowRight size={12} /></span></div>
        <div className="shelf-grid" data-crop="scrolling-catalog">
          {filtered.map((anime) => <button className="poster-card" onClick={() => choose(anime)} key={anime.id} data-target={`anime-${anime.id}`}>
            <span className="poster-wrap"><Poster anime={anime} />{follows[anime.id] !== undefined && <Badge icon={<Check size={10} />}>已追番</Badge>}<span className="poster-hover">了解这部 <ArrowRight size={14} /></span></span>
            <strong>{anime.title}</strong><span className="poster-meta">{DAYS[anime.day]}<span>{follows[anime.id] !== undefined ? '已在片单' : '查看详情'}</span></span>
          </button>)}
          {!filtered.length && noResults}
        </div>
      </div>}

      {direction === 'daily' && <div className="daily-content">
        <div className="page-heading"><div><span className="eyebrow">YOUR WEEK IN ANIME</span><h1>每天都有点期待。</h1><p>顺着放送日，看看这一季的故事。</p></div><SearchBox value={query} onChange={setQuery} /></div>
        {days()}
        <div className="daily-columns">
          <section className="day-list"><div className="day-list-heading"><h2>{day < 0 ? '全部放送' : `${DAYS[day]}放送`}</h2><span>{filtered.length} 部</span></div>
            <div className="daily-rows" data-crop="scrolling-schedule">{filtered.map((anime, index) => <Row key={anime.id} className="schedule-row" selected={anime.id === current.id} role="button" tabIndex={0} onClick={() => choose(anime)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); choose(anime) } }} data-target={`anime-${anime.id}`} lead={<Poster anime={anime} />} title={<strong>{anime.title}</strong>} subtitle={<span>{DAYS[anime.day]} · {follows[anime.id] !== undefined ? '已加入片单' : '尚未追番'}</span>} meta={follows[anime.id] !== undefined ? <Check size={13} /> : undefined}><ChevronRight size={14} /></Row>)}
              {!filtered.length && noResults}</div>
            <div className="schedule-footnote"><span className="quiet-dot" />排期为交互样例，不代表实时更新</div>
          </section>
          <section className="daily-detail">{filtered.length > 0 ? <><div className="daily-art"><Poster anime={current} /><div><span>本日片单</span><strong>{String(filtered.findIndex((anime) => anime.id === current.id) + 1).padStart(2, '0')}<small> / {String(filtered.length).padStart(2, '0')}</small></strong></div></div>{details(current, true)}</> : <div className="detail-empty"><CalendarDays size={30} /><p>换一天，看看别的故事。</p></div>}</section>
        </div>
      </div>}

      {direction === 'cinema' && <div className="cinema-content">
        <div className="cinema-header"><div><span className="eyebrow">PICK YOUR NEXT STORY</span><h1>今晚，去另一个世界。</h1></div><SearchBox value={query} onChange={setQuery} /></div>
        <div className="cinema-filter">{days(true)}<span>{filtered.length} 部作品</span></div>
        {filtered.length > 0 ? <>
          <section className="cinema-hero">
            <img className="cinema-backdrop" src={`/art/${current.cover}`} alt="" />
            <div className="cinema-shade" />
            <div className="cinema-copy"><span className="cinema-kicker"><span />{DAYS[current.day]} · 秋季片单</span><h2>{current.title}</h2><p>先被一张海报吸引，<br />再给一个新故事留些时间。</p><div className="cinema-actions"><Action variant="primary" onClick={startVersion} icon={currentFollowed ? <Check size={17} /> : <Heart size={17} />} target="start-follow">{currentFollowed ? '查看所选版本' : '选择版本追番'}</Action><span>选好版本，往后安心等更新</span></div></div>
            <Poster anime={current} className="cinema-feature-poster" />
            <div className="cinema-pagination"><span>{String(filtered.findIndex((anime) => anime.id === current.id) + 1).padStart(2, '0')} <small>/ {String(filtered.length).padStart(2, '0')}</small></span><Action variant="ghost" label="上一部" onClick={() => { const index = filtered.findIndex((anime) => anime.id === current.id); choose(filtered[(index - 1 + filtered.length) % filtered.length]) }} icon={<ChevronLeft size={17} />}>{null}</Action><Action variant="ghost" label="下一部" onClick={() => { const index = filtered.findIndex((anime) => anime.id === current.id); choose(filtered[(index + 1) % filtered.length]) }} icon={<ChevronRight size={17} />}>{null}</Action></div>
          </section>
          <div className="rail-heading"><span>接着发现</span><span>点选海报，切换主画面 <ArrowRight size={12} /></span></div>
          <div className="cinema-rail" data-crop="horizontal-poster-rail">{filtered.map((anime) => <button key={anime.id} className={`rail-card ${current.id === anime.id ? 'selected' : ''}`} onClick={() => choose(anime)} data-target={`anime-${anime.id}`}><Poster anime={anime} /><span>{anime.title}</span>{current.id === anime.id && <span className="rail-selected">正在预览</span>}</button>)}</div>
        </> : noResults}
      </div>}
      <div className="product-footer"><span>海报来自应用内置目录 · 排期与版本仅作演示</span><span>Dipper · 浏览概念</span></div>
    </main>

    {direction === 'shelf' && detail && <div className="sheet-layer" onMouseDown={(event) => { if (event.target === event.currentTarget) close() }}>
      <section className="detail-sheet" role="dialog" aria-modal="true" aria-label={versionOpen ? '选择追番版本' : selected.title} ref={dialog} onKeyDown={trap}>
        <div className="sheet-toolbar">{versionOpen ? <Action variant="ghost" icon={<ArrowLeft size={15} />} onClick={() => setVersionOpen(false)}>作品详情</Action> : <span>作品预览</span>}<Action variant="ghost" label="关闭详情" onClick={() => { setVersionOpen(false); setDetail(false) }} icon={<X size={18} />}>{null}</Action></div>
        <div className="sheet-scroll" data-crop="scrolling-detail">{versionOpen ? <><div className="sheet-version-anime"><Poster anime={selected} /><strong>{selected.title}</strong></div>{versionPicker}</> : <><div className="sheet-art"><img src={`/art/${selected.cover}`} className="sheet-backdrop" alt="" /><Poster anime={selected} /></div>{details(selected)}</>}</div>
      </section>
    </div>}

    {direction !== 'shelf' && versionOpen && <div className="dialog-layer" onMouseDown={(event) => { if (event.target === event.currentTarget) close() }}>
      <section className="version-dialog" role="dialog" aria-modal="true" aria-label="选择追番版本" ref={dialog} onKeyDown={trap}><div className="sheet-toolbar"><span>追番设置</span><Action variant="ghost" label="关闭版本选择" onClick={close} icon={<X size={18} />}>{null}</Action></div><div className="sheet-version-anime"><Poster anime={current} /><strong>{current.title}</strong></div>{versionPicker}</section>
    </div>}
    {notice && <div className="study-toast" role="status"><Check size={16} /><span>{notice}</span>{undoId && <button onClick={undo}>撤销</button>}<button onClick={() => setNotice('')} aria-label="关闭提示"><X size={14} /></button></div>}
  </Window>

  return <div id="example" className={`preview-stage ${capture ? 'capture-preview' : ''}`}><Stage width={1144} height={744} compact={{ below: 800, width: 358, height: 810 }} theme={direction === 'cinema' ? 'dark' : 'light'} fidelity="realistic" unframed tokens={{ '--ss-font': '"Segoe UI", "Microsoft YaHei UI", sans-serif', '--ss-accent': direction === 'daily' ? '#2b6958' : direction === 'cinema' ? '#d4e399' : '#3169ce' }}>{() => product}</Stage></div>
}

function Study() {
  const params = new URLSearchParams(location.search)
  const initial = params.get('capture') ?? location.hash.slice(1)
  const [direction, setDirection] = useState<Direction>(OPTIONS.some((option) => option.id === initial) ? initial as Direction : 'shelf')
  const [catalog, setCatalog] = useState<Anime[]>([])
  const [error, setError] = useState('')
  const [comparison, setComparison] = useState(false)
  useEffect(() => {
    fetch('/art/catalog.json').then((response) => { if (!response.ok) throw new Error('catalog'); return response.json() }).then((all: Anime[]) => setCatalog(PINNED.map((id) => all.find((anime) => anime.id === id)!).filter(Boolean))).catch(() => setError('海报目录未能载入，请刷新页面。'))
    const change = () => { const hash = location.hash.slice(1); if (OPTIONS.some((option) => option.id === hash)) setDirection(hash as Direction) }
    window.addEventListener('hashchange', change)
    return () => window.removeEventListener('hashchange', change)
  }, [])
  if (params.get('capture') === 'gold') return <div id="example" className="gold-host"><Gold /></div>
  if (error) return <div className="load-message">{error}</div>
  if (!catalog.length) return <div className="load-message">正在摆好这一季的海报…</div>
  const current = OPTIONS.find((option) => option.id === direction)!
  const switchDirection = (next: Direction) => { setDirection(next); history.replaceState(null, '', `#${next}`) }
  if (params.has('capture')) return <div className="capture-host"><AppPreview key={direction} direction={direction} catalog={catalog} capture /></div>
  return <div className="study-page">
    <header className="study-header"><a href="#shelf" className="study-brand" onClick={() => switchDirection('shelf')}><img src="/art/dipper.svg" alt="" /><strong>Dipper</strong><span>设计探索 / 浏览番剧</span></a><button onClick={() => setComparison(!comparison)} className="compare-toggle">{comparison ? '收起比较' : '三个方向怎么选'}<ArrowRight size={14} /></button></header>
    <div className="study-intro"><div><span className="study-overline">BROWSE, THREE WAYS</span><h1>同一季番剧，三种相遇方式。</h1></div><p>切换方向，点点海报，试一次追番。<br /><span>交互原型 · 不影响你的真实片单</span></p></div>
    <div className="direction-picker"><Tabs value={OPTIONS.findIndex((option) => option.id === direction)} items={OPTIONS.map((option) => <button key={option.id} onClick={() => switchDirection(option.id)} aria-pressed={direction === option.id}><span>{option.number}</span><option.icon size={17} />{option.name}</button>)} /></div>
    {comparison && <div className="comparison">{OPTIONS.map((option) => <button onClick={() => switchDirection(option.id)} key={option.id}><small>{option.number} / {option.flow}</small><strong>{option.name}</strong><p>{option.description}</p><span>{option.id === 'shelf' ? '适合：漫无目的地逛，慢慢发现喜欢的番' : option.id === 'daily' ? '适合：每天回来，明确找某一天的番' : '适合：想看点什么，但还没选定作品'}</span></button>)}</div>}
    <div className="direction-caption"><div><strong>{current.line}</strong><span>{current.description}</span></div><span className="flow-label">{current.flow}</span></div>
    <AppPreview key={direction} direction={direction} catalog={catalog} />
    <footer className="study-footer"><span>先感受浏览节奏，再决定细节。可以搜索、切星期、选版本、追番和撤销。</span><span>01 / 02 / 03 · 2026.11</span></footer>
  </div>
}

createRoot(document.getElementById('root')!).render(<Study />)
