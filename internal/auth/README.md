# Autenticação e Segurança (`internal/auth/`)

## O que é este diretório?
Módulo de autenticação OAuth2 com extensão PKCE e custódia segura de credenciais.

## Para que serve?
Gerencia o fluxo de login oficial no YouTube sem expor senhas ou chaves em arquivos de configuração, delegando a persistência ao Keyring do SO.
