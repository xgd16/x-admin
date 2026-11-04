/**
 * WebAuthn / 通行密钥的浏览器封装与错误提示。
 */
import type {
  PasskeyCreationOptionsJSON,
  PasskeyLoginOptions,
  PasskeyRegistrationOptions,
  PasskeyRequestOptionsJSON,
} from '../api/types'

/** 判断当前浏览器与访问地址是否支持 WebAuthn。 */
export function isPasskeySupported(): boolean {
  if (typeof window === 'undefined' || typeof navigator === 'undefined') return false
  if (typeof window.PublicKeyCredential === 'undefined' || !navigator.credentials) return false
  const host = window.location.hostname
  if (!host) return false
  // WebAuthn 的 RP ID 必须是域名，IP 字面量不能作为 RP ID。
  if (/^\d{1,3}(\.\d{1,3}){3}$/.test(host) || host.includes(':')) return false
  return window.isSecureContext !== false
}

/** 把 base64url 字符串解码为 ArrayBuffer，兼容缺失的 padding。 */
export function base64urlToBuffer(value: string): ArrayBuffer {
  const normalized = value.replace(/-/g, '+').replace(/_/g, '/')
  const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, '=')
  const binary = atob(padded)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i += 1) bytes[i] = binary.charCodeAt(i)
  return bytes.buffer
}

/** 把 ArrayBuffer 编码为无 padding 的 base64url，分块处理避免栈溢出。 */
export function bufferToBase64url(value: ArrayBuffer): string {
  const bytes = new Uint8Array(value)
  let binary = ''
  const chunk = 0x8000
  for (let i = 0; i < bytes.length; i += chunk) {
    binary += String.fromCharCode(...bytes.subarray(i, i + chunk))
  }
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '')
}

/** 把服务端 JSON 选项转换为浏览器注册 API 需要的结构。 */
function toCreationOptions(json: PasskeyCreationOptionsJSON): PublicKeyCredentialCreationOptions {
  return {
    rp: json.rp,
    user: { ...json.user, id: base64urlToBuffer(json.user.id) },
    challenge: base64urlToBuffer(json.challenge),
    pubKeyCredParams: json.pubKeyCredParams.map((param) => ({
      type: 'public-key',
      alg: param.alg,
    })),
    timeout: json.timeout,
    excludeCredentials: json.excludeCredentials?.map((credential) => ({
      type: 'public-key',
      id: base64urlToBuffer(credential.id),
      transports: credential.transports as AuthenticatorTransport[] | undefined,
    })),
    authenticatorSelection: json.authenticatorSelection
      ? {
          authenticatorAttachment: json.authenticatorSelection
            .authenticatorAttachment as AuthenticatorAttachment | undefined,
          requireResidentKey: json.authenticatorSelection.requireResidentKey,
          residentKey: json.authenticatorSelection.residentKey as ResidentKeyRequirement | undefined,
          userVerification: json.authenticatorSelection
            .userVerification as UserVerificationRequirement | undefined,
        }
      : undefined,
    attestation: json.attestation as AttestationConveyancePreference | undefined,
  }
}

/** 把服务端 JSON 选项转换为浏览器登录 API 需要的结构。 */
function toRequestOptions(json: PasskeyRequestOptionsJSON): PublicKeyCredentialRequestOptions {
  return {
    challenge: base64urlToBuffer(json.challenge),
    timeout: json.timeout,
    rpId: json.rpId,
    allowCredentials: json.allowCredentials?.map((credential) => ({
      type: 'public-key',
      id: base64urlToBuffer(credential.id),
      transports: credential.transports as AuthenticatorTransport[] | undefined,
    })),
    userVerification: json.userVerification as UserVerificationRequirement | undefined,
  }
}

/** 调用 navigator.credentials.create，并把注册结果序列化为后端可接收的 JSON。 */
export async function createPasskey(options: PasskeyRegistrationOptions): Promise<unknown> {
  if (!isPasskeySupported()) {
    throw new Error('当前环境不支持通行密钥')
  }
  const credential = (await navigator.credentials.create({
    publicKey: toCreationOptions(options.options.publicKey),
  })) as PublicKeyCredential | null
  if (!credential) throw new Error('未创建通行密钥')
  const response = credential.response as AuthenticatorAttestationResponse
  return {
    id: credential.id,
    rawId: bufferToBase64url(credential.rawId),
    type: credential.type,
    response: {
      clientDataJSON: bufferToBase64url(response.clientDataJSON),
      attestationObject: bufferToBase64url(response.attestationObject),
      transports: response.getTransports?.() ?? [],
    },
    clientExtensionResults: credential.getClientExtensionResults(),
    authenticatorAttachment: credential.authenticatorAttachment ?? undefined,
  }
}

/** 调用 navigator.credentials.get，并把登录断言序列化为后端可接收的 JSON。 */
export async function getPasskeyAssertion(options: PasskeyLoginOptions): Promise<unknown> {
  if (!isPasskeySupported()) {
    throw new Error('当前环境不支持通行密钥')
  }
  const credential = (await navigator.credentials.get({
    publicKey: toRequestOptions(options.options.publicKey),
  })) as PublicKeyCredential | null
  if (!credential) throw new Error('未返回通行密钥')
  const response = credential.response as AuthenticatorAssertionResponse
  return {
    id: credential.id,
    rawId: bufferToBase64url(credential.rawId),
    type: credential.type,
    response: {
      clientDataJSON: bufferToBase64url(response.clientDataJSON),
      authenticatorData: bufferToBase64url(response.authenticatorData),
      signature: bufferToBase64url(response.signature),
      userHandle: response.userHandle ? bufferToBase64url(response.userHandle) : undefined,
    },
    clientExtensionResults: credential.getClientExtensionResults(),
    authenticatorAttachment: credential.authenticatorAttachment ?? undefined,
  }
}

/** 把 WebAuthn 的 DOMException 转换成用户能理解的中文提示。 */
export function passkeyErrorMessage(error: unknown): string {
  if (typeof DOMException !== 'undefined' && error instanceof DOMException) {
    switch (error.name) {
      case 'NotAllowedError':
        return '操作已取消或超时，请重试'
      case 'SecurityError':
        return '当前访问地址不支持通行密钥，请使用 localhost 或 HTTPS 域名'
      case 'InvalidStateError':
        return '该设备已经绑定过了，请直接使用通行密钥登录'
      case 'NotSupportedError':
        return '当前设备或浏览器不支持通行密钥'
      case 'AbortError':
        return '操作已取消'
    }
  }
  if (error instanceof Error && error.message) return error.message
  return '通行密钥操作失败，请重试'
}
