/**
 * 文件库选择弹窗：类型筛选、搜索、上传与多选。
 */
import { useEffect, useRef, useState } from 'react'
import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query'
import { Button, Empty, Input, Modal, Pagination, Spin, Upload } from 'antd'
import { api } from '../api/client'
import type { FileItem, PageResult } from '../api/types'
import { useAuth } from '../stores/auth'
import { Icon } from './Icon'
import { formatSize } from '../utils/format'
import {
  FILE_KIND_LABELS,
  FILE_KIND_OPTIONS,
  acceptForKinds,
  fileKind,
  fileKindIcon,
  fileKindLabel,
  fileMatchesKinds,
  normalizeFileKinds,
  type FileKind,
} from '../utils/file'

const PAGE_SIZE = 24

export interface FilePickerProps {
  open: boolean
  /** 允许的文件类型，例如 `image` 或 `['image', 'video']`；不传表示全部。 */
  types?: FileKind | FileKind[]
  /** 是否允许多选，默认 false。 */
  multiple?: boolean
  /** 最多可选文件数，仅在多选时生效。 */
  limit?: number
  /** 预选文件，通常由 FileSelect 传入。 */
  value?: FileItem[]
  title?: string
  /** 即使当前账号有上传权限，也可以强制隐藏上传按钮。 */
  upload?: boolean
  onCancel: () => void
  onConfirm: (files: FileItem[]) => void
}

export function FilePicker({
  open,
  types,
  multiple = false,
  limit,
  value,
  title = '选择文件',
  upload,
  onCancel,
  onConfirm,
}: FilePickerProps) {
  const queryClient = useQueryClient()
  const canUpload = useAuth((state) => state.can('system:file:upload'))
  const allowUpload = upload ?? canUpload
  const [selected, setSelected] = useState<FileItem[]>([])
  const [keyword, setKeyword] = useState('')
  const [draft, setDraft] = useState('')
  const [page, setPage] = useState(1)
  const [activeKind, setActiveKind] = useState<FileKind | 'all'>('all')
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState('')
  // 用 ref 保存最新预选值，避免每次 value 变化都重置弹窗内部状态。
  const valueRef = useRef(value)
  valueRef.current = value

  const kinds = normalizeFileKinds(types)
  const kindKey = kinds.join(',')
  const typeOptions = kinds.length ? FILE_KIND_OPTIONS.filter((option) => kinds.includes(option.value)) : FILE_KIND_OPTIONS
  const max = multiple ? limit : 1
  const kindFilter = activeKind === 'all' ? kindKey || undefined : activeKind
  const uploadTypes = activeKind === 'all' ? types : activeKind

  // 每次打开弹窗时重置筛选、分页和临时选择。
  useEffect(() => {
    if (!open) return
    setSelected(valueRef.current ?? [])
    setKeyword('')
    setDraft('')
    setPage(1)
    setActiveKind('all')
    setUploading(false)
    setError('')
  }, [open])

  useEffect(() => {
    if (activeKind !== 'all' && !typeOptions.some((option) => option.value === activeKind)) {
      setActiveKind('all')
    }
  }, [activeKind, typeOptions])

  // 文件列表只在弹窗打开时请求；翻页时保留上一页数据，避免闪白。
  const query = useQuery({
    queryKey: ['files', 'picker', page, keyword, kindFilter],
    queryFn: () => api.get<PageResult<FileItem>>('/files', { page, page_size: PAGE_SIZE, keyword, kind: kindFilter }),
    enabled: open,
    placeholderData: keepPreviousData,
  })

  const rows = query.data?.list ?? []
  const total = query.data?.total ?? 0

  // 已有 URL 的预选文件可能缺少 id，列表返回后把它补全为完整文件记录。
  useEffect(() => {
    if (!open || !query.data) return
    setSelected((current) => {
      let changed = false
      const next = current.map((item) => {
        if (item.id) return item
        const match = query.data.list.find((row) => row.url === item.url)
        if (!match) return item
        changed = true
        return match
      })
      return changed ? next : current
    })
  }, [open, query.data])

  const isSelected = (file: FileItem) => selected.some((item) => (item.id ? item.id === file.id : item.url === file.url))

  const trimToLimit = (next: FileItem[]) => {
    if (max === undefined || max <= 0) return next
    return next.slice(0, max)
  }

  const toggle = (file: FileItem) => {
    setError('')
    const exists = isSelected(file)
    if (exists) {
      setSelected((current) => current.filter((item) => (item.id ? item.id !== file.id : item.url !== file.url)))
      return
    }
    if (!multiple) {
      setSelected([file])
      return
    }
    if (max !== undefined && max > 0 && selected.length >= max) {
      setError(`最多选择 ${max} 个文件`)
      return
    }
    setSelected((current) => [...current, file])
  }

  const remove = (file: FileItem) => {
    setError('')
    setSelected((current) => current.filter((item) => (item.id ? item.id !== file.id : item.url !== file.url)))
  }

  const applyKeyword = () => {
    setKeyword(draft.trim())
    setPage(1)
  }

  // 上传前先校验类型和数量，成功后刷新文件库并自动选中新文件。
  const handleUpload = async (file: File) => {
    setError('')
    if (!fileMatchesKinds({ name: file.name, mime: file.type }, uploadTypes)) {
      setError(`只能上传${fileKindLabel(uploadTypes)}`)
      return
    }
    if (multiple && max !== undefined && max > 0 && selected.length >= max) {
      setError(`最多选择 ${max} 个文件`)
      return
    }
    setUploading(true)
    try {
      const uploaded = await api.upload<FileItem>('/files', file)
      await queryClient.invalidateQueries({ queryKey: ['files'] })
      setSelected((current) => {
        const next = multiple ? [...current, uploaded] : [uploaded]
        return trimToLimit(next)
      })
    } catch (uploadError) {
      setError(uploadError instanceof Error ? uploadError.message : '上传失败')
    } finally {
      setUploading(false)
    }
  }

  const confirm = () => {
    if (!selected.length) return
    onConfirm(selected)
  }

  const showTabs = typeOptions.length > 1

  return (
    <Modal
      open={open}
      width={920}
      title={
        <div className="file-picker-title">
          <span className="file-picker-kicker">FILE LIBRARY</span>
          <b>{title}</b>
          <span className="file-picker-scope">{fileKindLabel(types)}</span>
        </div>
      }
      footer={null}
      onCancel={onCancel}
      destroyOnHidden
      mask={{ closable: false }}
      className="file-picker-modal"
    >
      <div className="file-picker">
        <div className="file-picker-toolbar">
          {showTabs ? (
            <div className="file-picker-tabs" role="tablist" aria-label="文件类型">
              <button
                type="button"
                role="tab"
                aria-selected={activeKind === 'all'}
                className={activeKind === 'all' ? 'active' : ''}
                onClick={() => {
                  setActiveKind('all')
                  setPage(1)
                }}
              >
                全部
              </button>
              {typeOptions.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  role="tab"
                  aria-selected={activeKind === option.value}
                  className={activeKind === option.value ? 'active' : ''}
                  onClick={() => {
                    setActiveKind(option.value)
                    setPage(1)
                  }}
                >
                  <Icon name={option.icon} size={12} /> {option.label}
                </button>
              ))}
            </div>
          ) : (
            <span className="file-picker-scope-chip">
              <Icon name={typeOptions[0]?.icon ?? 'folder'} size={12} /> {fileKindLabel(types)}
            </span>
          )}
          <div className="file-picker-search">
            <Input
              allowClear
              placeholder="搜索文件名 / MIME"
              prefix={<Icon name="search" size={12} />}
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              onPressEnter={applyKeyword}
            />
            <Button onClick={applyKeyword}>搜索</Button>
          </div>
          {allowUpload && (
            <Upload
              accept={acceptForKinds(uploadTypes) || undefined}
              multiple={multiple}
              showUploadList={false}
              disabled={uploading}
              beforeUpload={(file) => {
                void handleUpload(file)
                return false
              }}
            >
              <Button loading={uploading}>
                <Icon name="upload" size={12} /> 上传
              </Button>
            </Upload>
          )}
        </div>

        {selected.length > 0 && (
          <div className="file-picker-selected">
            <span className="file-picker-selected-label">已选 {selected.length}{max && max > 0 ? ` / ${max}` : ''}</span>
            <div className="file-picker-chips">
              {selected.map((file) => (
                <span className="file-picker-chip" key={file.id || file.url}>
                  <span>{file.name}</span>
                  <button type="button" aria-label={`移除 ${file.name}`} onClick={() => remove(file)}>
                    <Icon name="x" size={9} />
                  </button>
                </span>
              ))}
            </div>
          </div>
        )}

        {error && <div className="file-picker-error">{error}</div>}

        <div className="file-picker-body">
          {query.isLoading ? (
            <div className="file-picker-state">
              <Spin size="small" />
              <span>正在读取文件库</span>
            </div>
          ) : query.isError ? (
            <div className="file-picker-state error">
              <Icon name="file" size={22} />
              <span>{query.error instanceof Error ? query.error.message : '文件库加载失败'}</span>
              <Button size="small" onClick={() => void query.refetch()}>
                重试
              </Button>
            </div>
          ) : rows.length === 0 ? (
            <div className="file-picker-state">
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description={keyword ? '没有匹配的文件' : '文件库还是空的'} />
              {allowUpload && !keyword && (
                <span className="file-picker-hint">可以直接上传第一个文件，上传后会立即选中。</span>
              )}
            </div>
          ) : (
            <div className="file-picker-grid">
              {rows.map((file) => {
                const selectedRow = isSelected(file)
                const kind = fileKind(file)
                return (
                  <button
                    type="button"
                    key={file.id}
                    className={`file-card${selectedRow ? ' selected' : ''}`}
                    aria-pressed={selectedRow}
                    title={file.name}
                    onClick={() => toggle(file)}
                    onDoubleClick={() => {
                      if (!multiple) onConfirm([file])
                    }}
                  >
                    <span className="file-card-thumb">
                      {kind === 'image' && file.url ? (
                        <img src={file.url} alt={file.name} loading="lazy" />
                      ) : (
                        <span className={`file-card-kind ${kind}`}>
                          <Icon name={fileKindIcon(file)} size={22} />
                          <em>{(file.ext || FILE_KIND_LABELS[kind]).toUpperCase()}</em>
                        </span>
                      )}
                      {selectedRow && (
                        <span className="file-card-check">
                          <Icon name="check" size={11} />
                        </span>
                      )}
                    </span>
                    <span className="file-card-meta">
                      <b>{file.name}</b>
                      <span>
                        {formatSize(file.size)} · {FILE_KIND_LABELS[kind]}
                      </span>
                    </span>
                  </button>
                )
              })}
            </div>
          )}
        </div>

        <div className="file-picker-footer">
          <span className="file-picker-count">
            共 {total} 个文件
            {selected.length > 0 ? ` · 已选 ${selected.length}` : ''}
          </span>
          {total > PAGE_SIZE && (
            <Pagination
              size="small"
              current={page}
              pageSize={PAGE_SIZE}
              total={total}
              showSizeChanger={false}
              onChange={(next) => setPage(next)}
            />
          )}
          <div className="file-picker-actions">
            <Button onClick={onCancel}>取消</Button>
            <Button type="primary" disabled={!selected.length} onClick={confirm}>
              {multiple ? `使用所选 ${selected.length} 项` : '使用所选文件'}
            </Button>
          </div>
        </div>
      </div>
    </Modal>
  )
}
