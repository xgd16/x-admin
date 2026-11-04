/**
 * 表单内文件选择器，直接兼容 antd Form.Item。
 */
import { useState } from 'react'
import type { FileItem } from '../api/types'
import { FilePicker } from './FilePicker'
import { Icon } from './Icon'
import {
  FILE_KIND_OPTIONS,
  extensionOf,
  fileKind,
  fileKindIcon,
  fileKindLabel,
  fileNameFromURL,
  normalizeFileKinds,
  type FileKind,
} from '../utils/file'

export interface FileSelectProps {
  /** URL 或 URL 数组，可直接接入 antd Form.Item。 */
  value?: string | string[] | null
  onChange?: (value: string | string[]) => void
  /** 允许的文件类型，例如 `image` 或 `['image', 'video']`；不传表示全部。 */
  types?: FileKind | FileKind[]
  multiple?: boolean
  limit?: number
  disabled?: boolean
  placeholder?: string
  title?: string
  upload?: boolean
}

/** 把已有 URL 包装成 FileItem，便于复用文件卡片渲染。 */
function urlItem(url: string): FileItem {
  const name = fileNameFromURL(url)
  return {
    id: 0,
    name,
    key: '',
    url,
    mime: '',
    ext: extensionOf(name),
    size: 0,
    uploader_id: 0,
    created_at: '',
  }
}

export function FileSelect({
  value,
  onChange,
  types,
  multiple = false,
  limit,
  disabled = false,
  placeholder,
  title,
  upload,
}: FileSelectProps) {
  const [open, setOpen] = useState(false)
  const [items, setItems] = useState<FileItem[]>([])

  // 统一成数组处理；单选时只保留第一个值。
  const rawUrls = Array.isArray(value) ? value.filter(Boolean) : value ? [value] : []
  const urls = multiple ? (limit && limit > 0 ? rawUrls.slice(0, limit) : rawUrls) : rawUrls.slice(0, 1)
  const entries = urls.map((url) => items.find((item) => item.url === url) ?? urlItem(url))
  const full = multiple && !!limit && limit > 0 && entries.length >= limit
  const canAdd = !disabled && (multiple ? !full : entries.length === 0)
  const kinds = normalizeFileKinds(types)
  const addIcon = kinds.length === 1 ? FILE_KIND_OPTIONS.find((option) => option.value === kinds[0])?.icon : 'plus'
  const label = placeholder ?? `选择${fileKindLabel(types)}`

  // 对 Form.Item 只回传 URL，保持表单值是可序列化的字符串。
  const emit = (next: FileItem[]) => {
    setItems(next)
    onChange?.(multiple ? next.map((item) => item.url) : next[0]?.url ?? '')
  }

  const remove = (url: string) => {
    emit(entries.filter((item) => item.url !== url))
  }

  return (
    <>
      <div className={`file-select${disabled ? ' disabled' : ''}`}>
        {entries.map((item) => {
          const kind = fileKind(item)
          return (
            <div
              className={`file-select-item ${kind}`}
              key={item.url}
              role="button"
              tabIndex={disabled ? -1 : 0}
              title={`${item.name} · 点击更换`}
              onClick={() => {
                if (!disabled) setOpen(true)
              }}
              onKeyDown={(event) => {
                if (!disabled && (event.key === 'Enter' || event.key === ' ')) {
                  event.preventDefault()
                  setOpen(true)
                }
              }}
            >
              {kind === 'image' ? (
                <img src={item.url} alt={item.name} />
              ) : (
                <span className={`file-select-kind ${kind}`}>
                  <Icon name={fileKindIcon(item)} size={20} />
                  <em>{(item.ext || item.name).toUpperCase()}</em>
                </span>
              )}
              {!disabled && (
                <button
                  type="button"
                  className="file-select-remove"
                  aria-label={`移除 ${item.name}`}
                  title="移除"
                  onClick={(event) => {
                    event.stopPropagation()
                    remove(item.url)
                  }}
                >
                  <Icon name="x" size={9} />
                </button>
              )}
            </div>
          )
        })}
        {canAdd && (
          <button type="button" className="file-select-add" onClick={() => setOpen(true)}>
            <Icon name={addIcon ?? 'plus'} size={18} />
            <span>{label}</span>
          </button>
        )}
      </div>

      <FilePicker
        open={open}
        types={types}
        multiple={multiple}
        limit={limit}
        value={entries}
        title={title ?? label}
        upload={upload}
        onCancel={() => setOpen(false)}
        onConfirm={(files) => {
          setOpen(false)
          emit(files)
        }}
      />
    </>
  )
}
