import React, { useEffect, useLayoutEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react'
import { createRoot } from 'react-dom/client'
import { ArrowDownToLine, ArrowLeft, ArrowRight, BookOpen, CalendarDays, Check, ChevronDown, ChevronLeft, ChevronRight, Compass, Grid2X2, Heart, ListFilter, Search, Settings2, Sparkles, X } from 'lucide-react'
import { Badge, Button, Field, Row, Stage, States, Tabs, Window, type Act } from './kit'
import { useReducedMotion } from './kit/reduced-motion'
import Gold from './gold'
import MotionGold from './motion-gold'
import './study.css'

type Direction = 'shelf' | 'daily' | 'cinema'
type Anime = { id: string; title: string; day: number; cover: string }
const WORK_DETAILS: Record<string, { originalTitle: string; synopsis: string[] }> = {
  '4031': {
    originalTitle: '天は赤い河のほとり',
    synopsis: [
      '一名日本中学生原本过着平凡的校园生活，某天却被水中突然伸出的手拖入异世界。醒来时，她已经身处古代赫梯帝国，被带到了王妃面前。',
      '王妃想用夕梨的血完成诅咒，让自己的儿子登上王位。生死关头，第三王子出手相救。为了寻找回到现代的方法，夕梨与王子一同行动，也逐渐卷入宫廷的权力斗争。',
    ],
  },
  '4063': {
    originalTitle: '魔法少女育成計画 restart',
    synopsis: [
      '一位擅长用魔法制作美食的少女，原本过着为心仪男生准备便当的日常。某天，她与另外十五位魔法少女被强行带入了一个来历不明的游戏空间。',
      '十六人各有不同的魔法。故事从她们被传送进游戏的那一天展开，原本熟悉的魔法与陌生的游戏世界交织在一起。',
    ],
  },
}
const DAYS = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']
const OPTIONS: { id: Direction; number: string; name: string; line: string; description: string; flow: string; icon: typeof Grid2X2 }[] = [
  { id: 'shelf', number: '01', name: '海报书架', line: '慢慢逛，遇到想看的。', description: '从海报进入作品详情。看过简介，再决定追不追；关上后，仍在刚才的位置。', flow: '扫海报 → 了解作品 → 选字幕组', icon: Grid2X2 },
  { id: 'daily', number: '02', name: '每日放送', line: '这一天，有什么可看？', description: '用星期组织这一季。左边快速扫片，右边直接看详情，适合每天回来找更新。', flow: '选星期 → 切换作品 → 选字幕组', icon: CalendarDays },
  { id: 'cinema', number: '03', name: '沉浸选片', line: '先被一部作品打动。', description: '一次只聚焦一部。大幅画面与下方片单联动，少一点管理感，多一点选片的期待。', flow: '翻片单 → 聚焦作品 → 选字幕组', icon: Sparkles },
]
type Release = { episode: string; spec: string; size: string; when: string }
type GroupOption = { name: string; count: number; releases: Release[] }
const GROUP_OPTIONS: GroupOption[] = [
  { name: 'ANi', count: 12, releases: [
    { episode: '第 06 话', spec: '1080P · MKV · 简体', size: '1.4 GB', when: '今天 21:10' },
    { episode: '第 05 话', spec: '1080P · MKV · 简体', size: '1.4 GB', when: '昨天 20:48' },
    { episode: '第 04 话', spec: '1080P · MKV · 简体', size: '1.3 GB', when: '11 月 9 日' },
  ] },
  { name: '银色子弹字幕组', count: 9, releases: [
    { episode: '第 06 话', spec: '1080P · MP4 · 简繁', size: '1.1 GB', when: '今天 20:36' },
    { episode: '第 05 话', spec: '1080P · MP4 · 简繁', size: '1.1 GB', when: '昨天 20:12' },
    { episode: '第 04 话', spec: '1080P · MP4 · 简繁', size: '1.0 GB', when: '11 月 9 日' },
  ] },
  { name: 'LoliHouse', count: 7, releases: [
    { episode: '第 06 话', spec: '1080P · MKV · 简繁', size: '1.6 GB', when: '今天 19:54' },
    { episode: '第 05 话', spec: '1080P · MKV · 简繁', size: '1.6 GB', when: '昨天 19:30' },
    { episode: '第 04 话', spec: '1080P · MKV · 简繁', size: '1.5 GB', when: '11 月 9 日' },
  ] },
  { name: '幻樱字幕组', count: 4, releases: [
    { episode: '第 05 话', spec: '720P · MP4 · 简体', size: '620 MB', when: '昨天 22:16' },
    { episode: '第 04 话', spec: '720P · MP4 · 简体', size: '620 MB', when: '11 月 9 日' },
    { episode: '第 03 话', spec: '720P · MP4 · 简体', size: '610 MB', when: '11 月 2 日' },
  ] },
]
const REHEARSAL_GROUP = 0
const PINNED = ['4063', '4031', '3887', '4059', '4056', '4070', '4105', '4072', '4086', '4062', '4082', '4118', '4085', '4057', '4093', '4087', '3984', '4058', '4068']

function Action({ children, onClick, variant = 'secondary', className = '', icon, target, label, disabled = false }: { children: ReactNode; onClick: () => void; variant?: 'primary' | 'secondary' | 'ghost' | 'tertiary'; className?: string; icon?: ReactNode; target?: string; label?: string; disabled?: boolean }) {
  return <Button variant={variant} size="large" className={className} icon={icon} onClick={() => { if (!disabled) onClick() }} tabIndex={0} aria-label={label} aria-disabled={disabled || undefined} data-target={target} onKeyDown={(event) => {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      if (!disabled) onClick()
    }
  }}>{children}</Button>
}

function SearchBox({ value, onChange, onEnter }: { value: string; onChange: (value: string) => void; onEnter?: () => void }) {
  const input = useRef<HTMLInputElement>(null)
  return <div className="search-box">
    <Field value={value} placeholder="搜索番剧" leading={<Search size={15} />} />
    <input ref={input} value={value} onChange={(event) => onChange(event.target.value)} placeholder="搜索番剧" aria-label="搜索番剧" onKeyDown={(event) => {
      if (event.nativeEvent.isComposing) return
      if (event.key === 'Enter') onEnter?.()
      if (event.key === 'Escape') { event.stopPropagation(); onChange('') }
    }} />
    {value && <button className="search-clear" onClick={() => { onChange(''); input.current?.focus() }} aria-label="清空搜索"><X size={13} /></button>}
  </div>
}

function Poster({ anime, className = '' }: { anime: Anime; className?: string }) {
  return <img className={`poster ${className}`} src={`/art/${anime.cover}`} alt={anime.title} draggable={false} />
}

function Sidebar({ direction, count, onOther, inactive }: { direction: Direction; count: number; onOther: () => void; inactive: boolean }) {
  return <aside className="app-sidebar" {...(inactive ? { inert: '' } : {})}>
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

type RehearsalState = { beat: number }
const REHEARSAL_START: RehearsalState = { beat: 0 }
const REHEARSAL_POSTER: RehearsalState = { beat: 4 }

async function interactiveStage(act: Act<RehearsalState>) {
  await act.wait(0)
}

async function discoveryStory(act: Act<RehearsalState>) {
  act.mark('shelf')
  await act.wait(1000)
  await act.click('anime-4031')
  await act.set({ beat: 1 })
  act.mark('reading')
  await act.wait(2200)
  await act.click('choose-group')
  await act.set({ beat: 2 })
  await act.wait(700)
  await act.click({ at: 'group-0', x: 0.18, y: 0.4 })
  await act.set({ beat: 3 })
  await act.wait(900)
  await act.click({ at: 'confirm-follow', x: 0.88, y: 0.8 })
  await act.set({ beat: 4 })
  act.mark('followed')
  await act.wait(2600)
  await act.click('undo-follow')
  await act.set({ beat: 5 })
  await act.wait(900)
  await act.click('close-detail')
  await act.set({ beat: 6 })
  act.mark('returned')
  await act.wait(1800)
}

function RehearsalDriver({ beat, actions }: { beat: number; actions: React.MutableRefObject<Partial<Record<number | 'poster', () => void>>> }) {
  const reduced = useReducedMotion()
  useLayoutEffect(() => { actions.current[reduced ? 'poster' : beat]?.() }, [beat, reduced])
  return null
}

function ReadingScene({ anime, followed, onScroll }: { anime: Anime; followed: boolean; onScroll?: (position: number) => void }) {
  const work = WORK_DETAILS[anime.id]
  return <>
    <div className="detail-cover"><Poster anime={anime} /></div>
    <div className="detail-identity">
      <h2>{anime.title}</h2>
      {work && <p className="detail-original" lang="ja">{work.originalTitle}</p>}
      <div className="detail-metadata"><span>{DAYS[anime.day]} · 演示排期</span><span className="detail-followed" data-visible={followed || undefined} aria-hidden={!followed}><Check size={14} />已追番</span></div>
    </div>
    <div className="detail-reading" role="region" aria-label={`${anime.title}剧情简介`} tabIndex={0} data-crop="scrolling-detail" onScroll={(event) => onScroll?.(event.currentTarget.scrollTop)}>
      <section className="detail-synopsis" aria-label="剧情简介"><h3>剧情简介</h3><div className="detail-prose">{work ? work.synopsis.map((paragraph) => <p key={paragraph}>{paragraph}</p>) : <p className="detail-unavailable">暂时没有这部作品的简介。</p>}</div></section>
      {work && <p className="detail-source">简介据{anime.id === '4031' ? '蜜柑计划作品资料' : '官方第一话介绍'}整理</p>}
    </div>
  </>
}

function ReadingTransition({ anime, followed }: { anime: Anime; followed: boolean }) {
  const reduced = useReducedMotion()
  const last = useRef({ anime, followed, scrollTop: 0 })
  const [outgoing, setOutgoing] = useState<typeof last.current | null>(null)
  useLayoutEffect(() => {
    if (last.current.anime.id !== anime.id && !reduced) setOutgoing(last.current)
    last.current = { anime, followed, scrollTop: last.current.anime.id === anime.id ? last.current.scrollTop : 0 }
    const timer = setTimeout(() => setOutgoing(null), 180)
    return () => clearTimeout(timer)
  }, [anime, followed, reduced])
  return <div className="detail-scenes">
    {outgoing && !reduced && <div className="detail-panel-body detail-outgoing" aria-hidden="true" {...{ inert: '' }} ref={(element) => { const reading = element?.querySelector<HTMLElement>('.detail-reading'); if (reading) reading.scrollTop = outgoing.scrollTop }}><ReadingScene {...outgoing} /></div>}
    <div key={anime.id} className="detail-panel-body detail-current"><ReadingScene anime={anime} followed={followed} onScroll={(position) => { last.current.scrollTop = position }} /></div>
  </div>
}

function GroupWorkspace({ anime, choice, saved, onChoose }: { anime: Anime; choice: number | null; saved?: number; onChoose: (choice: number) => void }) {
  const [query, setQuery] = useState('')
  const groups = GROUP_OPTIONS.map((group, index) => ({ ...group, index })).filter((group) => group.name.toLowerCase().includes(query.trim().toLowerCase()))
  const group = choice === null ? null : GROUP_OPTIONS[choice]
  return <div className="group-workspace">
    <div className="group-work-identity"><Poster anime={anime} /><div><h2>{anime.title}</h2><p>选择字幕组，再看看它发布的资源。</p></div></div>
    <div className="group-columns">
      <section className="group-list-pane" aria-label="字幕组">
        <h3>字幕组 <small>{GROUP_OPTIONS.length} 组</small></h3>
        <label className="group-search"><Search size={14} /><input value={query} onChange={(event) => setQuery(event.target.value)} aria-label="搜索字幕组" placeholder="搜索字幕组" /></label>
        <div className="group-options" role="radiogroup" aria-label="选择字幕组">
          {groups.map((option, position) => <button key={option.name} role="radio" aria-checked={choice === option.index} tabIndex={choice === option.index || (!groups.some((item) => item.index === choice) && position === 0) ? 0 : -1} className={`group-option ${choice === option.index ? 'chosen' : ''}`} data-target={`group-${option.index}`} onClick={() => onChoose(option.index)} onKeyDown={(event) => {
            if (!['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End'].includes(event.key)) return
            event.preventDefault()
            const next = event.key === 'Home' ? 0 : event.key === 'End' ? groups.length - 1 : (position + (['ArrowRight', 'ArrowDown'].includes(event.key) ? 1 : -1) + groups.length) % groups.length
            onChoose(groups[next].index)
            event.currentTarget.parentElement?.querySelector<HTMLElement>(`[data-target="group-${groups[next].index}"]`)?.focus()
          }}><span><strong>{option.name}</strong><small>{option.count} 个资源{saved === option.index ? ' · 正在追' : ''}</small></span><span className="group-selected-mark"><Check size={14} /></span></button>)}
          {!groups.length && <p className="group-no-results">没有匹配的字幕组，换个名字试试。</p>}
        </div>
      </section>
      <section className="group-resource-pane" aria-label="可下载资源" aria-live="polite">
        {group && <div className="group-resource-content" key={group.name}><div className="group-resource-head"><h3>可下载资源 <small>{group.name}</small></h3><p>最近发布 · 共 {group.count} 个资源</p></div><div className="release-list">{group.releases.map((release) => <div className="release-row" key={release.episode}><div><strong>{release.episode}</strong><span>{release.spec}</span></div><div><span>{release.size}</span><small>{release.when}</small></div></div>)}</div><p className="group-collection-note">确认后，自动收集这个字幕组后续发布的资源。</p></div>}
        <div className="group-resource-empty" data-visible={!group || undefined} aria-hidden={!!group} {...(group ? { inert: '' } : {})}><BookOpen size={24} /><h3>先选一个字幕组</h3><p>在这里预览它发布的资源，<br />再决定要不要追。</p></div>
      </section>
    </div>
  </div>
}

function AppPreview({ direction, catalog, capture = false, rehearsal = false }: { direction: Direction; catalog: Anime[]; capture?: boolean; rehearsal?: boolean }) {
  const reduced = useReducedMotion()
  const [day, setDay] = useState(direction === 'daily' ? 2 : -1)
  const [query, setQuery] = useState('')
  const initialAnime = catalog.find((anime) => anime.id === new URLSearchParams(location.search).get('anime'))
  const [selected, setSelected] = useState(initialAnime ?? catalog[0])
  const [detail, setDetail] = useState(direction === 'shelf' && !!initialAnime)
  const [closing, setClosing] = useState(false)
  const [groupOpen, setGroupOpen] = useState(false)
  const [groupExiting, setGroupExiting] = useState(false)
  const previousGroupOpen = useRef(false)
  const [groupChoice, setGroupChoice] = useState<number | null>(null)
  const [follows, setFollows] = useState<Record<string, number>>({ '4056': 0, '4072': 1 })
  const [notice, setNotice] = useState('')
  const [undoId, setUndoId] = useState<string | null>(null)
  const [onlyUnfollowed, setOnlyUnfollowed] = useState(false)
  const [seasonOpen, setSeasonOpen] = useState(false)
  const dialog = useRef<HTMLElement>(null)
  const setupPage = useRef<HTMLElement>(null)
  const detailOpener = useRef<HTMLElement | null>(null)
  const openerIndex = useRef(0)
  const groupOpener = useRef<HTMLElement | null>(null)
  const noticeTimer = useRef<ReturnType<typeof setTimeout>>()
  const lastNotice = useRef('')
  if (notice) lastNotice.current = notice
  const closeTimer = useRef<ReturnType<typeof setTimeout>>()
  const grid = useRef<HTMLDivElement>(null)
  const detailItems = useRef<Anime[]>(catalog)
  const rehearsalActions = useRef<Partial<Record<number | 'poster', () => void>>>({})
  const search = query.trim().toLowerCase()
  const filtered = catalog.filter((anime) => direction === 'shelf' && search
    ? anime.title.toLowerCase().includes(search)
    : (day < 0 || anime.day === day) && anime.title.toLowerCase().includes(search) && (!onlyUnfollowed || follows[anime.id] === undefined))
  const isFollowed = follows[selected.id] !== undefined
  const visibleSelected = filtered.find((anime) => anime.id === selected.id) ?? filtered[0] ?? selected
  const current = direction === 'shelf' ? selected : visibleSelected
  const currentFollowed = follows[current.id] !== undefined
  const count = Object.keys(follows).length
  const comparisonItems = detail ? detailItems.current : filtered
  const selectedIndex = comparisonItems.findIndex((anime) => anime.id === selected.id)

  useLayoutEffect(() => {
    const exiting = previousGroupOpen.current && !groupOpen && !reduced
    previousGroupOpen.current = groupOpen
    setGroupExiting(exiting)
    if (!exiting) return
    const timer = setTimeout(() => setGroupExiting(false), 280)
    return () => clearTimeout(timer)
  }, [groupOpen, reduced])

  useEffect(() => {
    if (direction === 'daily') setSelected(catalog.find((anime) => anime.day === 2) ?? catalog[0])
    return () => { clearTimeout(noticeTimer.current); clearTimeout(closeTimer.current) }
  }, [])

  useEffect(() => {
    if (!detail) return
    detailOpener.current = document.activeElement !== document.body ? document.activeElement as HTMLElement : document.querySelector<HTMLElement>(`[data-target="anime-${selected.id}"]`)
    const frame = requestAnimationFrame(() => dialog.current?.querySelector<HTMLElement>(direction === 'shelf' ? '[data-target="close-detail"]' : '[role="button"], button')?.focus())
    return () => {
      cancelAnimationFrame(frame)
      const posters = grid.current?.querySelectorAll<HTMLElement>('.poster-card')
      const fallback = posters?.[Math.min(openerIndex.current, posters.length - 1)] ?? grid.current?.closest('main')?.querySelector<HTMLElement>('input[aria-label="搜索番剧"]')
      const opener = detailOpener.current?.isConnected ? detailOpener.current : fallback
      opener?.focus({ preventScroll: true })
    }
  }, [detail])

  useEffect(() => {
    if (direction !== 'shelf') return
    const shortcut = (event: globalThis.KeyboardEvent) => {
      if (detail || groupOpen || event.altKey || event.isComposing) return
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
        event.preventDefault()
        grid.current?.closest('main')?.querySelector<HTMLInputElement>('input[aria-label="搜索番剧"]')?.focus()
      }
    }
    document.addEventListener('keydown', shortcut)
    return () => document.removeEventListener('keydown', shortcut)
  }, [detail, groupOpen, direction])

  useLayoutEffect(() => {
    if (reduced || !grid.current) return
    const animation = grid.current.animate([{ opacity: .65, transform: 'translateY(4px)' }, { opacity: 1, transform: 'translateY(0)' }], { duration: 160, easing: 'ease-out' })
    return () => animation.cancel()
  }, [day, query, onlyUnfollowed, reduced])

  useEffect(() => {
    if (!groupOpen) return
    groupOpener.current = document.activeElement as HTMLElement
    const frame = requestAnimationFrame(() => setupPage.current?.querySelector<HTMLElement>('[data-target="back-to-work"]')?.focus())
    return () => {
      cancelAnimationFrame(frame)
      if (direction === 'shelf') dialog.current?.querySelector<HTMLElement>('[data-target="choose-group"]')?.focus()
      else if (groupOpener.current?.isConnected) groupOpener.current.focus()
    }
  }, [groupOpen])

  useEffect(() => {
    if (!detail && !groupOpen) return
    const dismiss = (event: globalThis.KeyboardEvent) => {
      if (event.key !== 'Escape') return
      if (groupOpen) setGroupOpen(false)
      else dismissDetail()
    }
    document.addEventListener('keydown', dismiss)
    return () => document.removeEventListener('keydown', dismiss)
  }, [detail, groupOpen, closing, reduced])

  const inform = (message: string, undo: string | null = null) => {
    clearTimeout(noticeTimer.current)
    setNotice(message)
    setUndoId(undo)
    noticeTimer.current = setTimeout(() => {
      if ((document.activeElement as HTMLElement)?.dataset.target === 'undo-follow') dialog.current?.querySelector<HTMLElement>('[data-target="confirm-follow"]')?.focus()
      setNotice('')
      setUndoId(null)
    }, 6500)
  }
  const choose = (anime: Anime) => {
    if (!detail) { detailItems.current = filtered; openerIndex.current = filtered.findIndex((item) => item.id === anime.id) }
    clearTimeout(closeTimer.current)
    setClosing(false)
    setSelected(anime)
    setGroupOpen(false)
    setGroupChoice(follows[anime.id] ?? null)
    if (direction === 'shelf') setDetail(true)
  }
  const commitFollow = (anime: Anime, choice: number) => {
    if (follows[anime.id] === choice) return
    const replacing = follows[anime.id] !== undefined
    setFollows((previous) => ({ ...previous, [anime.id]: choice }))
    setGroupOpen(false)
    inform(replacing ? `已更换《${anime.title}》的字幕组` : `已将《${anime.title}》加入演示片单`, replacing ? null : anime.id)
  }
  const follow = () => { if (groupOpen && groupChoice !== null) commitFollow(current, groupChoice) }
  const undo = () => {
    if (!undoId) return
    setFollows((previous) => { const next = { ...previous }; delete next[undoId]; return next })
    setUndoId(null)
    setGroupChoice(null)
    if (direction === 'shelf') setGroupOpen(false)
    inform('已撤销，本地应用中的追番未受影响')
  }
  const startGroup = () => {
    setSelected(current)
    setGroupChoice(follows[current.id] ?? null)
    setGroupOpen(true)
  }
  const close = () => {
    if (groupOpen) { setGroupOpen(false); return }
    dismissDetail()
  }
  function dismissDetail() {
    if (closing) return
    if (reduced) { setDetail(false); setClosing(false); return }
    setClosing(true)
    closeTimer.current = setTimeout(() => { setDetail(false); setClosing(false) }, 180)
  }
  const trap = (event: KeyboardEvent) => {
    if (event.key === 'Escape') { event.stopPropagation(); close() }
    if (event.key !== 'Tab' || !dialog.current) return
    const elements = Array.from(dialog.current.querySelectorAll<HTMLElement>('button:not(:disabled):not([tabindex="-1"]), [tabindex="0"], input'))
    const first = elements[0]
    const last = elements[elements.length - 1]
    if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus() }
    else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus() }
  }

  const weekdayValues = [-1, 1, 2, 3, 4, 5, 6, 0]
  const days = (short = false) => <div className={`weekday-tabs ${short ? 'short-days' : ''}`} role={direction === 'shelf' ? 'group' : 'tablist'} aria-label="按星期筛选">
    {weekdayValues.map((value, index) => <button key={value} role={direction === 'shelf' ? undefined : 'tab'} aria-selected={direction === 'shelf' ? undefined : day === value} aria-pressed={direction === 'shelf' ? !search && day === value : undefined} tabIndex={direction === 'shelf' && day !== value ? -1 : 0} className={(direction !== 'shelf' || !search) && day === value ? 'active' : ''} onKeyDown={(event) => {
      if (direction !== 'shelf' || !['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return
      event.preventDefault()
      const next = event.key === 'Home' ? 0 : event.key === 'End' ? weekdayValues.length - 1 : (index + (event.key === 'ArrowRight' ? 1 : -1) + weekdayValues.length) % weekdayValues.length
      setQuery('')
      setDay(weekdayValues[next])
      event.currentTarget.parentElement?.querySelector<HTMLElement>(`[data-target="day-${weekdayValues[next]}"]`)?.focus()
    }} onClick={() => { if (direction === 'shelf') setQuery(''); setDay(value); setDetail(false) }} data-target={`day-${value}`}>
      {value === -1 ? '全部' : DAYS[value]}{direction === 'daily' && value !== -1 && <small>{catalog.filter((anime) => anime.day === value).length}</small>}
    </button>)}
  </div>

  const season = <div className="season-wrap">
    <button className="season-button" onClick={() => setSeasonOpen(!seasonOpen)} aria-expanded={seasonOpen}>2026 秋季<ChevronDown size={14} /></button>
    {seasonOpen && <div className="season-popover"><strong>季度示意</strong><p>本次使用本地海报样本。</p><p>先体验布局，季度切换暂未接入。</p><Action variant="ghost" onClick={() => setSeasonOpen(false)}>知道了</Action></div>}
  </div>

  const unchanged = currentFollowed && follows[current.id] === groupChoice
  const groupConfirmation = <Action variant="primary" className="confirm-follow" onClick={follow} disabled={groupChoice === null || unchanged} target="confirm-follow" label={unchanged ? '已在片单' : currentFollowed ? '更换字幕组' : '确认追番'} icon={<Heart size={16} />}>{unchanged ? '已在片单' : currentFollowed ? '更换字幕组' : '确认追番'}</Action>
  const groupPicker = <GroupWorkspace key={current.id} anime={current} choice={groupChoice} saved={follows[current.id]} onChoose={setGroupChoice} />
  const groupSummary = Object.fromEntries(GROUP_OPTIONS.map((group, index) => [String(index), group.name]))

  const details = (anime: Anime, compact = false) => <div className={`anime-details ${compact ? 'compact-detail' : ''}`}>
    <span className="eyebrow">{DAYS[anime.day]}放送 · 秋季片单</span>
    <h2>{anime.title}</h2>
    <div className="detail-tags"><Badge>{DAYS[anime.day]}</Badge><Badge>电视动画</Badge>{follows[anime.id] !== undefined && <Badge tone="success" icon={<Check size={11} />}>已追番</Badge>}</div>
    <p className="detail-copy">把想看的留在片单里。选一个字幕组，看看它发布的资源，再决定追番。</p>
    <div className="detail-facts"><span>收集方式<strong>跟随所选字幕组</strong></span><span>片单归属<strong>我的追番</strong></span></div>
    <Action variant="primary" onClick={startGroup} icon={follows[anime.id] !== undefined ? <Check size={16} /> : <Heart size={16} />} target="start-follow">{follows[anime.id] !== undefined ? '追番设置' : '追这部'}</Action>
    <p className="helper-copy">{follows[anime.id] !== undefined ? '已留在片单里，前往「我的追番」接着看。' : '先了解作品，再选字幕组。'}</p>
  </div>

  const noResults = <div className="empty-state"><Search size={27} /><h3>{query ? '没找到这部番' : '这一天，暂时没有样本'}</h3><p>{query ? '换个关键词，或者看看这一季的其他作品。' : '试试周二或周三，继续体验排期浏览。'}</p><Action onClick={() => { setQuery(''); setDay(-1); setOnlyUnfollowed(false) }} variant="secondary">查看全部番剧</Action></div>

  const product = <Window className={`study-window ${direction}`} title="Dipper" toolbar={<div className="windows-actions" aria-hidden="true"><span>—</span><span>□</span><span>×</span></div>} body={{ className: 'product-body' }}>
    <Sidebar direction={direction} count={count} onOther={() => inform('这里先专注「浏览番剧」，其他页面暂未接入')} inactive={detail && !groupOpen} />
    <main className="browse-main" aria-label={`${OPTIONS.find((option) => option.id === direction)?.name}方向`} {...(detail && !groupOpen ? { inert: '' } : {})}>
      <div className="browse-page" hidden={groupOpen} {...(groupOpen ? { inert: '' } : {})}>
      <div className="product-top"><span><Compass size={14} /> 浏览番剧</span><div className="top-season">{season}</div><span className="sample-label">本地海报样本</span></div>
      {direction === 'shelf' && <div className="shelf-content">
        <div className="page-heading"><div><span className="eyebrow">2026 · AUTUMN COLLECTION</span><h1>这一季，想看什么？</h1><p>从一张海报开始，找到下一部期待。</p></div><SearchBox value={query} onChange={setQuery} onEnter={() => { if (search && filtered.length === 1) choose(filtered[0]) }} /></div>
        <div className="shelf-filter">{days()}<button className={`unfollowed-filter ${!search && onlyUnfollowed ? 'active' : ''}`} aria-pressed={!search && onlyUnfollowed} onClick={() => { setQuery(''); setOnlyUnfollowed(!onlyUnfollowed) }}><ListFilter size={14} />未追番</button></div>
        <div className="catalog-subline"><span role="status">{search ? '全季搜索' : day < 0 ? '秋季番剧' : `${DAYS[day]}片单`}<small>{filtered.length} 部作品</small></span><span>{search ? '搜索覆盖整季，清空后恢复筛选' : '点开海报，先了解一下'} <ArrowRight size={12} /></span></div>
        <div className="shelf-grid" ref={grid} data-crop="scrolling-catalog">
          {filtered.map((anime) => <button className="poster-card" onClick={() => choose(anime)} key={anime.id} data-target={`anime-${anime.id}`}>
            <span className="poster-wrap"><Poster anime={anime} /><span className="poster-followed" data-visible={follows[anime.id] !== undefined || undefined} aria-hidden={follows[anime.id] === undefined}><Badge>已追番</Badge></span><span className="poster-hover">了解这部</span></span>
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
            <div className="cinema-copy"><span className="cinema-kicker"><span />{DAYS[current.day]} · 秋季片单</span><h2>{current.title}</h2><p>先被一张海报吸引，<br />再给一个新故事留些时间。</p><div className="cinema-actions"><Action variant="primary" onClick={startGroup} icon={currentFollowed ? <Check size={17} /> : <Heart size={17} />} target="start-follow">{currentFollowed ? '追番设置' : '追这部'}</Action><span>选好字幕组，往后安心等更新</span></div></div>
            <Poster anime={current} className="cinema-feature-poster" />
            <div className="cinema-pagination"><span>{String(filtered.findIndex((anime) => anime.id === current.id) + 1).padStart(2, '0')} <small>/ {String(filtered.length).padStart(2, '0')}</small></span><Action variant="ghost" label="上一部" onClick={() => { const index = filtered.findIndex((anime) => anime.id === current.id); choose(filtered[(index - 1 + filtered.length) % filtered.length]) }} icon={<ChevronLeft size={17} />}>{null}</Action><Action variant="ghost" label="下一部" onClick={() => { const index = filtered.findIndex((anime) => anime.id === current.id); choose(filtered[(index + 1) % filtered.length]) }} icon={<ChevronRight size={17} />}>{null}</Action></div>
          </section>
          <div className="rail-heading"><span>接着发现</span><span>点选海报，切换主画面 <ArrowRight size={12} /></span></div>
          <div className="cinema-rail" data-crop="horizontal-poster-rail">{filtered.map((anime) => <button key={anime.id} className={`rail-card ${current.id === anime.id ? 'selected' : ''}`} onClick={() => choose(anime)} data-target={`anime-${anime.id}`}><Poster anime={anime} /><span>{anime.title}</span>{current.id === anime.id && <span className="rail-selected">正在预览</span>}</button>)}</div>
        </> : noResults}
      </div>}
      <div className="product-footer"><span>海报来自应用内置目录 · 排期与字幕组资源仅作演示</span><span>Dipper · 浏览概念</span></div>
      </div>
      {(groupOpen || groupExiting) && <section className="follow-setup" data-exiting={groupExiting || undefined} aria-label="追番设置" aria-hidden={groupExiting || undefined} {...(groupExiting ? { inert: '' } : {})} ref={setupPage}>
        <header className="setup-heading"><Action variant="ghost" target="back-to-work" onClick={() => setGroupOpen(false)} icon={<ChevronLeft size={16} />}>{direction === 'shelf' ? '返回详情' : '返回作品'}</Action><span>追番设置</span></header>
        <div className="setup-content">{groupPicker}</div>
        <footer className="detail-panel-footer"><div className="detail-action-note"><span>{groupChoice === null ? '先选一个字幕组，看看它的资源' : `将跟随 ${GROUP_OPTIONS[groupChoice].name} 更新`}</span><small>演示数据 · 不会创建真实订阅或下载</small></div><div className="detail-panel-actions">{groupConfirmation}</div></footer>
      </section>}
    </main>

    {direction === 'shelf' && detail && <div className="sheet-layer" data-yielding={groupOpen || undefined} aria-hidden={groupOpen || undefined} {...(groupOpen ? { inert: '' } : {})} data-closing={closing || undefined} onMouseDown={(event) => { if (event.target === event.currentTarget) close() }}>
      <section className="detail-panel" role="dialog" aria-modal="true" aria-label={selected.title} ref={dialog} onKeyDown={trap}>
        <div className="sheet-toolbar"><span>作品详情</span><div className="detail-navigation"><button data-target="detail-prev" aria-label="上一部" disabled={selectedIndex <= 0} onClick={() => choose(comparisonItems[selectedIndex - 1])}><ChevronLeft size={16} /></button><span>{selectedIndex < 0 ? '—' : selectedIndex + 1} / {comparisonItems.length}</span><button data-target="detail-next" aria-label="下一部" disabled={selectedIndex < 0 || selectedIndex >= comparisonItems.length - 1} onClick={() => choose(comparisonItems[selectedIndex + 1])}><ChevronRight size={16} /></button></div><Action variant="ghost" target="close-detail" label="关闭详情" onClick={dismissDetail} icon={<X size={18} />}>{null}</Action></div>
        <ReadingTransition anime={selected} followed={isFollowed} />
        <footer className="detail-panel-footer">
          <div className="detail-action-note" role="status"><span><States value={isFollowed ? 'followed' : 'new'} align="start" states={{ followed: '已加入我的追番', new: '想看这部？留在片单里' }} /><button className="undo-action" data-target="undo-follow" data-visible={undoId === selected.id || undefined} onClick={undo} disabled={undoId !== selected.id} aria-hidden={undoId !== selected.id} tabIndex={undoId === selected.id ? 0 : -1}>撤销</button></span><small><States value={isFollowed ? String(follows[selected.id]) : 'new'} align="start" states={{ new: '演示原型 · 不影响真实片单', ...groupSummary }} /></small></div>
          <div className="detail-panel-actions"><Action variant="ghost" onClick={dismissDetail}>继续逛逛</Action><Action variant="primary" target="choose-group" onClick={startGroup} icon={<Heart size={16} />}>{isFollowed ? '追番设置' : '追这部'}</Action></div>
        </footer>
      </section>
    </div>}

    <div className="study-toast" role="status" data-visible={!!notice && !(direction === 'shelf' && detail) || undefined} aria-hidden={!notice || (direction === 'shelf' && detail)} {...(!notice || (direction === 'shelf' && detail) ? { inert: '' } : {})}><Check size={16} /><span>{lastNotice.current}</span>{undoId && <button data-target="toast-undo-follow" onClick={undo}>撤销</button>}<button onClick={() => setNotice('')} aria-label="关闭提示"><X size={14} /></button></div>
  </Window>

  rehearsalActions.current = {
    0: () => { setDetail(false); setClosing(false); setGroupOpen(false); setGroupChoice(null); setFollows({ '4056': 0, '4072': 1 }); setNotice(''); setUndoId(null) },
    1: () => choose(catalog.find((anime) => anime.id === '4031')!),
    2: startGroup,
    3: () => setGroupChoice(REHEARSAL_GROUP),
    4: follow,
    5: undo,
    6: dismissDetail,
    poster: () => {
      const anime = catalog.find((item) => item.id === '4031')!
      choose(anime)
      setGroupOpen(false)
      setGroupChoice(REHEARSAL_GROUP)
      setFollows({ '4056': 0, '4072': 1, '4031': 0 })
    },
  }
  return <div id="example" className={`preview-stage ${capture ? 'capture-preview' : ''}`}><Stage width={1144} height={744} compact={{ below: 800, width: 358, height: 810 }} theme={direction === 'cinema' ? 'dark' : 'light'} fidelity="realistic" unframed initial={REHEARSAL_START} poster={REHEARSAL_POSTER} script={rehearsal ? discoveryStory : interactiveStage} play="once" cursor={rehearsal ? { at: 'anime-4031', ripple: true } : undefined} tokens={{ '--ss-font': '"Segoe UI", "Microsoft YaHei UI", sans-serif', '--ss-accent': direction === 'daily' ? '#2b6958' : direction === 'cinema' ? '#d4e399' : '#3169ce' }}>{(state) => <>{rehearsal && <RehearsalDriver beat={state.beat} actions={rehearsalActions} />}{product}</>}</Stage></div>
}

function Study() {
  const params = new URLSearchParams(location.search)
  const initial = params.get('capture') ?? location.hash.slice(1)
  const [direction, setDirection] = useState<Direction>(OPTIONS.some((option) => option.id === initial) ? initial as Direction : 'shelf')
  const [catalog, setCatalog] = useState<Anime[]>([])
  const [error, setError] = useState('')
  const [comparison, setComparison] = useState(false)
  const [replay, setReplay] = useState(0)
  useEffect(() => {
    fetch('/art/catalog.json').then((response) => { if (!response.ok) throw new Error('catalog'); return response.json() }).then((all: Anime[]) => setCatalog(PINNED.map((id) => all.find((anime) => anime.id === id)!).filter(Boolean))).catch(() => setError('海报目录未能载入，请刷新页面。'))
    const change = () => { const hash = location.hash.slice(1); if (OPTIONS.some((option) => option.id === hash)) setDirection(hash as Direction) }
    window.addEventListener('hashchange', change)
    return () => window.removeEventListener('hashchange', change)
  }, [])
  if (params.get('capture') === 'gold') return <div id="example" className="gold-host"><Gold /></div>
  if (params.get('capture') === 'motion-gold') return <div id="example" className="gold-host"><MotionGold /></div>
  if (error) return <div className="load-message">{error}</div>
  if (!catalog.length) return <div className="load-message">正在摆好这一季的海报…</div>
  const current = OPTIONS.find((option) => option.id === direction)!
  const switchDirection = (next: Direction) => { setDirection(next); history.replaceState(null, '', `#${next}`) }
  if (params.has('capture')) return <div className="capture-host">{params.has('rehearse') && <div className="rehearsal-toolbar"><span>找番 → 了解 → 追番 · 交互演示</span><button onClick={() => setReplay(replay + 1)}>重新演示</button></div>}<AppPreview key={`${direction}-${replay}`} direction={direction} catalog={catalog} capture rehearsal={params.has('rehearse')} /></div>
  return <div className="study-page">
    <header className="study-header"><a href="#shelf" className="study-brand" onClick={() => switchDirection('shelf')}><img src="/art/dipper.svg" alt="" /><strong>Dipper</strong><span>设计探索 / 浏览番剧</span></a><button onClick={() => setComparison(!comparison)} className="compare-toggle">{comparison ? '收起比较' : '三个方向怎么选'}<ArrowRight size={14} /></button></header>
    <div className="study-intro"><div><span className="study-overline">BROWSE, THREE WAYS</span><h1>同一季番剧，三种相遇方式。</h1></div><p>切换方向，点点海报，试一次追番。<br /><span>交互原型 · 不影响你的真实片单</span></p></div>
    <div className="direction-picker"><Tabs value={OPTIONS.findIndex((option) => option.id === direction)} items={OPTIONS.map((option) => <button key={option.id} onClick={() => switchDirection(option.id)} aria-pressed={direction === option.id}><span>{option.number}</span><option.icon size={17} />{option.name}</button>)} /></div>
    {comparison && <div className="comparison">{OPTIONS.map((option) => <button onClick={() => switchDirection(option.id)} key={option.id}><small>{option.number} / {option.flow}</small><strong>{option.name}</strong><p>{option.description}</p><span>{option.id === 'shelf' ? '适合：漫无目的地逛，慢慢发现喜欢的番' : option.id === 'daily' ? '适合：每天回来，明确找某一天的番' : '适合：想看点什么，但还没选定作品'}</span></button>)}</div>}
    <div className="direction-caption"><div><strong>{current.line}</strong><span>{current.description}</span></div><span className="flow-label">{current.flow}</span></div>
    <AppPreview key={direction} direction={direction} catalog={catalog} />
    <footer className="study-footer"><span>先感受浏览节奏，再决定细节。可以搜索、切星期、选字幕组、看资源、追番和撤销。</span><span>01 / 02 / 03 · 2026.11</span></footer>
  </div>
}

createRoot(document.getElementById('root')!).render(<Study />)
