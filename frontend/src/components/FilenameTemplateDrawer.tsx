/**
 * 导出文件名配置抽屉：片段开关、拖拽排序、时间格式与实时预览。
 * 模板为全局一份，存储在后端参数表（export.filename_template）。
 */
import { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button, Drawer, Input, Select } from 'antd'
import dayjs from 'dayjs'
import { api } from '../api/client'
import type { FilenameSegment, FilenameSegmentType, FilenameTemplate } from '../api/types'
import { useToast } from '../hooks/useToast'
import { Icon } from './Icon'

/** 时间片段格式白名单，value 与后端 FilenameTimeFormats 完全一致。 */
const TIME_FORMATS = [
  { value: '20060102150405', label: '20261108195800', dayjs: 'YYYYMMDDHHmmss' },
  { value: '2006-01-02', label: '2026-11-08', dayjs: 'YYYY-MM-DD' },
  { value: '2006-01-02_1504', label: '2026-11-08_1958', dayjs: 'YYYY-MM-DD_HHmm' },
  { value: '2006-01-02_15-04-05', label: '2026-11-08_19-58-00', dayjs: 'YYYY-MM-DD_HH-mm-ss' },
]

const SEGMENT_META: Record<FilenameSegmentType, { label: string; sample: string }> = {
  title: { label: '任务标题', sample: '用户列表' },
  module: { label: '模块标识', sample: 'users' },
  time: { label: '时间戳', sample: '' },
  text: { label: '自定义文本', sample: '' },
}

/** 按当前片段列表拼出预览文件名，与后端 ComposeFilename 行为一致。 */
function composePreview(segments: FilenameSegment[], now: dayjs.Dayjs): string {
  const parts: string[] = []
  for (const seg of segments) {
    if (seg.type === 'title') parts.push('用户列表')
    if (seg.type === 'module') parts.push('users')
    if (seg.type === 'time') {
      const fmt = TIME_FORMATS.find((f) => f.value === (seg.format || TIME_FORMATS[0].value))
      parts.push(now.format(fmt?.dayjs ?? 'YYYYMMDDHHmmss'))
    }
    if (seg.type === 'text' && seg.value) parts.push(seg.value)
  }
  return parts.filter(Boolean).join('_') || 'users'
}

interface FilenameTemplateDrawerProps {
  open: boolean
  onClose: () => void
}

export function FilenameTemplateDrawer({ open, onClose }: FilenameTemplateDrawerProps) {
  const toast = useToast()
  const queryClient = useQueryClient()
  const [segments, setSegments] = useState<FilenameSegment[]>([])
  const [dirty, setDirty] = useState(false)
  const [dragIndex, setDragIndex] = useState<number | null>(null)
  const [overIndex, setOverIndex] = useState<number | null>(null)

  const query = useQuery({
    queryKey: ['export-filename-template'],
    queryFn: () => api.get<FilenameTemplate>('/export-tasks/filename-template'),
    enabled: open,
  })

  // 打开抽屉时以服务端配置初始化本地编辑态。
  useEffect(() => {
    if (open && query.data) {
      setSegments(query.data.segments)
      setDirty(false)
    }
  }, [open, query.data])

  const mutate = (next: FilenameSegment[]) => {
    setSegments(next)
    setDirty(true)
  }

  const reorder = (from: number, to: number) => {
    if (from === to) return
    const next = [...segments]
    const [moved] = next.splice(from, 1)
    next.splice(to, 0, moved)
    mutate(next)
  }

  const addSegment = (type: FilenameSegmentType) => {
    if (type === 'text') {
      mutate([...segments, { type: 'text', value: '导出' }])
      return
    }
    if (type === 'time') {
      mutate([...segments, { type: 'time', format: TIME_FORMATS[0].value }])
      return
    }
    mutate([...segments, { type }])
  }

  const countOf = (type: FilenameSegmentType) => segments.filter((seg) => seg.type === type).length

  const saveMutation = useMutation({
    mutationFn: () => api.put('/export-tasks/filename-template', { segments }),
    onSuccess: () => {
      setDirty(false)
      void queryClient.invalidateQueries({ queryKey: ['export-filename-template'] })
      toast.success('文件名模板已保存，对下一次导出生效')
      onClose()
    },
    onError: (error: Error) => toast.fail(error),
  })

  const resetToDefault = () => {
    mutate([
      { type: 'module' },
      { type: 'time', format: TIME_FORMATS[0].value },
    ])
  }

  const preview = `${composePreview(segments, dayjs())}.xlsx`

  return (
    <Drawer
      open={open}
      onClose={onClose}
      size={430}
      title="导出文件名配置"
      extra={
        <Button size="small" type="text" onClick={resetToDefault}>
          恢复默认
        </Button>
      }
    >
      <p style={{ marginTop: 0, color: 'var(--text-2, #666)', fontSize: 12 }}>
        文件名由片段按顺序拼接，拖动手柄调整顺序；时间与自定义文本决定防重名能力。
      </p>
      <div
        className="mono"
        style={{
          padding: '8px 12px', borderRadius: 8, background: 'var(--glass-soft, #f5f5f5)',
          fontSize: 12, marginBottom: 12, wordBreak: 'break-all',
        }}
      >
        预览：{preview}
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        {segments.map((seg, index) => (
          <div
            key={`${seg.type}-${index}`}
            onDragOver={(event) => { event.preventDefault(); setOverIndex(index) }}
            onDrop={() => { if (dragIndex !== null) reorder(dragIndex, index); setDragIndex(null); setOverIndex(null) }}
            style={{
              display: 'flex', alignItems: 'center', gap: 8, padding: '6px 10px',
              borderRadius: 10, border: '1px solid var(--line, #e5e5e5)',
              opacity: dragIndex === index ? 0.45 : 1,
              outline: overIndex === index && dragIndex !== null && dragIndex !== index ? '1.5px dashed var(--accent, #1677ff)' : undefined,
            }}
          >
            <span
              draggable
              onDragStart={() => setDragIndex(index)}
              onDragEnd={() => { setDragIndex(null); setOverIndex(null) }}
              title="拖动排序"
              style={{ cursor: 'grab', lineHeight: 1, userSelect: 'none', touchAction: 'none' }}
            >
              ⠿
            </span>
            <b style={{ fontSize: 12, width: 64, flexShrink: 0 }}>{SEGMENT_META[seg.type].label}</b>
            {seg.type === 'text' && (
              <Input
                size="small"
                value={seg.value}
                maxLength={32}
                onChange={(event) => mutate(segments.map((s, i) => (i === index ? { ...s, value: event.target.value } : s)))}
                style={{ flex: 1 }}
              />
            )}
            {seg.type === 'time' && (
              <Select
                size="small"
                value={seg.format || TIME_FORMATS[0].value}
                options={TIME_FORMATS.map((f) => ({ label: f.label, value: f.value }))}
                onChange={(value) => mutate(segments.map((s, i) => (i === index ? { ...s, format: value } : s)))}
                style={{ flex: 1 }}
              />
            )}
            {seg.type !== 'text' && seg.type !== 'time' && <span style={{ flex: 1, fontSize: 12, color: 'var(--text-3, #999)' }}>{SEGMENT_META[seg.type].sample}</span>}
            <Button
              size="small"
              type="text"
              danger
              title="删除片段"
              disabled={segments.length <= 1}
              onClick={() => mutate(segments.filter((_, i) => i !== index))}
            >
              <Icon name="trash" size={12} />
            </Button>
          </div>
        ))}
      </div>

      <div style={{ display: 'flex', gap: 6, marginTop: 12, flexWrap: 'wrap' }}>
        {(['title', 'module', 'time', 'text'] as FilenameSegmentType[]).map((type) => {
          const maxed = type !== 'text' && countOf(type) >= 1
          return (
            <Button key={type} size="small" disabled={maxed || segments.length >= 8} onClick={() => addSegment(type)}>
              <Icon name="plus" size={11} /> {SEGMENT_META[type].label}
            </Button>
          )
        })}
      </div>

      <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8, marginTop: 20 }}>
        <Button onClick={onClose}>取消</Button>
        <Button type="primary" loading={saveMutation.isPending} disabled={!dirty || segments.length === 0} onClick={() => saveMutation.mutate()}>
          保存
        </Button>
      </div>
    </Drawer>
  )
}
