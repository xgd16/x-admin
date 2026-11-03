import type { FileItem } from '../api/types'

export type FileKind = 'image' | 'video' | 'audio' | 'document' | 'archive' | 'other'

export interface FileKindOption {
  value: FileKind
  label: string
  icon: string
  accept: string
}

export const FILE_KIND_OPTIONS: FileKindOption[] = [
  { value: 'image', label: '图片', icon: 'image', accept: 'image/*' },
  { value: 'video', label: '视频', icon: 'video', accept: 'video/*' },
  { value: 'audio', label: '音频', icon: 'music', accept: 'audio/*' },
  {
    value: 'document',
    label: '文档',
    icon: 'file',
    accept: '.pdf,.doc,.docx,.xls,.xlsx,.ppt,.pptx,.txt,.md,.csv,.json,.xml,.yaml,.yml,.html,.htm,.rtf',
  },
  { value: 'archive', label: '压缩包', icon: 'archive', accept: '.zip,.rar,.7z,.tar,.gz,.bz2,.xz' },
  { value: 'other', label: '其他', icon: 'file', accept: '*/*' },
]

export const FILE_KIND_LABELS: Record<FileKind, string> = {
  image: '图片',
  video: '视频',
  audio: '音频',
  document: '文档',
  archive: '压缩包',
  other: '其他',
}

const IMAGE_EXTS = new Set(['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg', 'bmp', 'ico', 'avif', 'heic', 'heif', 'tif', 'tiff'])
const VIDEO_EXTS = new Set(['mp4', 'webm', 'mov', 'm4v', 'avi', 'mkv', 'flv', 'wmv', 'mpeg', 'mpg'])
const AUDIO_EXTS = new Set(['mp3', 'wav', 'ogg', 'flac', 'aac', 'm4a', 'wma', 'opus'])
const DOCUMENT_EXTS = new Set(['pdf', 'doc', 'docx', 'xls', 'xlsx', 'ppt', 'pptx', 'txt', 'md', 'csv', 'json', 'xml', 'yaml', 'yml', 'html', 'htm', 'rtf'])
const ARCHIVE_EXTS = new Set(['zip', 'rar', '7z', 'tar', 'gz', 'bz2', 'xz'])

const DOCUMENT_MIMES = new Set(['application/pdf', 'application/json', 'application/xml', 'application/rtf'])

export interface FileIdentity {
  name?: string
  mime?: string
  ext?: string
}

export function extensionOf(name?: string): string {
  if (!name) return ''
  const clean = name.split(/[?#]/, 1)[0]
  const index = clean.lastIndexOf('.')
  return index >= 0 ? clean.slice(index + 1).toLowerCase() : ''
}

export function fileKind(file: FileIdentity): FileKind {
  const mime = (file.mime ?? '').toLowerCase()
  const ext = (file.ext ?? extensionOf(file.name)).toLowerCase()

  if (IMAGE_EXTS.has(ext) || mime.startsWith('image/')) return 'image'
  if (VIDEO_EXTS.has(ext) || mime.startsWith('video/')) return 'video'
  if (AUDIO_EXTS.has(ext) || mime.startsWith('audio/')) return 'audio'
  if (
    DOCUMENT_EXTS.has(ext) ||
    mime.startsWith('text/') ||
    DOCUMENT_MIMES.has(mime) ||
    mime.startsWith('application/vnd.ms-') ||
    mime.startsWith('application/vnd.openxmlformats-officedocument.')
  ) {
    return 'document'
  }
  if (ARCHIVE_EXTS.has(ext)) return 'archive'
  return 'other'
}

export function normalizeFileKinds(types?: FileKind | FileKind[] | null): FileKind[] {
  const values = types === undefined || types === null ? [] : Array.isArray(types) ? types : [types]
  const seen = new Set<FileKind>()
  values.forEach((value) => {
    if (FILE_KIND_LABELS[value]) seen.add(value)
  })
  const kinds = FILE_KIND_OPTIONS.map((option) => option.value).filter((value) => seen.has(value))
  return kinds.length === FILE_KIND_OPTIONS.length ? [] : kinds
}

export function fileKindQuery(types?: FileKind | FileKind[] | null): string | undefined {
  const kinds = normalizeFileKinds(types)
  return kinds.length ? kinds.join(',') : undefined
}

export function fileMatchesKinds(file: FileIdentity, types?: FileKind | FileKind[] | null): boolean {
  const kinds = normalizeFileKinds(types)
  if (kinds.length === 0) return true
  return kinds.includes(fileKind(file))
}

export function acceptForKinds(types?: FileKind | FileKind[] | null): string {
  const kinds = normalizeFileKinds(types)
  if (kinds.length === 0 || kinds.includes('other')) return ''
  return kinds.map((kind) => FILE_KIND_OPTIONS.find((option) => option.value === kind)?.accept ?? '').filter(Boolean).join(',')
}

export function fileKindLabel(types?: FileKind | FileKind[] | null): string {
  const kinds = normalizeFileKinds(types)
  if (kinds.length === 0) return '全部文件'
  return kinds.map((kind) => FILE_KIND_LABELS[kind]).join(' / ')
}

export function fileKindIcon(file: FileIdentity): string {
  return FILE_KIND_OPTIONS.find((option) => option.value === fileKind(file))?.icon ?? 'file'
}

export function fileNameFromURL(url: string): string {
  try {
    const path = url.split(/[?#]/, 1)[0]
    const name = decodeURIComponent(path.slice(path.lastIndexOf('/') + 1))
    return name || '未命名文件'
  } catch {
    return url
  }
}

export function selectedFileNames(files: FileItem[]): string {
  return files.map((file) => file.name).join('、')
}
