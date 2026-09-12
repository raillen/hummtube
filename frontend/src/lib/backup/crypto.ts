/**
 * Criptografia AES-GCM/PBKDF2 para backups pessoais no WebView.
 *
 * O backend exporta dados pessoais em JSON. Esta camada adiciona uma
 * criptografia opcional (AES-256-GCM com derivação PBKDF2-SHA256, 100k iterações)
 * controlada pelo usuário. A chave nunca é armazenada; o arquivo gerado
 * inclui o salt aleatório no início do payload.
 */

const PBKDF2_ITERATIONS = 100000;
const KEY_LENGTH_BITS = 256;
const SALT_LENGTH = 16;
const IV_LENGTH = 12;

async function deriveKey(passphrase: string, salt: Uint8Array): Promise<CryptoKey> {
  const enc = new TextEncoder();
  const baseKey = await crypto.subtle.importKey(
    'raw',
    enc.encode(passphrase),
    { name: 'PBKDF2' },
    false,
    ['deriveKey'],
  );
  return crypto.subtle.deriveKey(
    { name: 'PBKDF2', salt: salt as BufferSource, iterations: PBKDF2_ITERATIONS, hash: 'SHA-256' },
    baseKey,
    { name: 'AES-GCM', length: KEY_LENGTH_BITS },
    false,
    ['encrypt', 'decrypt'],
  );
}

export interface EncryptedPayload {
  v: 1;
  alg: 'AES-GCM-PBKDF2-SHA256';
  iter: number;
  salt: string;
  iv: string;
  ct: string;
}

export async function encryptBackupJSON(json: string, passphrase: string): Promise<string> {
  if (!passphrase || passphrase.length < 6) {
    throw new Error('A senha precisa ter pelo menos 6 caracteres.');
  }
  const salt = crypto.getRandomValues(new Uint8Array(SALT_LENGTH));
  const iv = crypto.getRandomValues(new Uint8Array(IV_LENGTH));
  const key = await deriveKey(passphrase, salt);
  const enc = new TextEncoder();
  const ciphertext = new Uint8Array(
    await crypto.subtle.encrypt({ name: 'AES-GCM', iv: iv as BufferSource }, key, enc.encode(json)),
  );
  const payload: EncryptedPayload = {
    v: 1,
    alg: 'AES-GCM-PBKDF2-SHA256',
    iter: PBKDF2_ITERATIONS,
    salt: bytesToBase64(salt),
    iv: bytesToBase64(iv),
    ct: bytesToBase64(ciphertext),
  };
  return JSON.stringify(payload);
}

export async function decryptBackupJSON(payload: string, passphrase: string): Promise<string> {
  const parsed = JSON.parse(payload) as EncryptedPayload;
  if (parsed.alg !== 'AES-GCM-PBKDF2-SHA256') {
    throw new Error('Formato de backup criptografado desconhecido.');
  }
  const salt = base64ToBytes(parsed.salt);
  const iv = base64ToBytes(parsed.iv);
  const ciphertext = base64ToBytes(parsed.ct);
  const key = await deriveKey(passphrase, salt);
  const dec = new TextDecoder();
  const plaintext = await crypto.subtle.decrypt({ name: 'AES-GCM', iv: iv as BufferSource }, key, ciphertext as BufferSource);
  return dec.decode(plaintext);
}

function bytesToBase64(bytes: Uint8Array): string {
  let binary = '';
  for (let i = 0; i < bytes.length; i += 1) binary += String.fromCharCode(bytes[i]);
  return btoa(binary);
}

function base64ToBytes(b64: string): Uint8Array {
  const binary = atob(b64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i += 1) bytes[i] = binary.charCodeAt(i);
  return bytes;
}

export function isEncryptedPayload(value: string): boolean {
  try {
    const parsed = JSON.parse(value) as Partial<EncryptedPayload>;
    return parsed.alg === 'AES-GCM-PBKDF2-SHA256' && typeof parsed.ct === 'string';
  } catch {
    return false;
  }
}
