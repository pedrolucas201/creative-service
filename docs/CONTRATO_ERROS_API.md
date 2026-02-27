# Contrato de Erros da API

Data de referencia: 27/02/2026

Este documento define o padrao oficial de resposta de erro do backend.

## 1) Objetivo

Padronizar respostas para:

- permitir UX clara no frontend;
- manter codigo estavel para tratamento por `error_code`;
- preservar compatibilidade com clientes existentes.

## 2) Formato de resposta

Para erros HTTP (4xx/5xx), a API retorna:

```json
{
  "error": "missing_authorization_header",
  "error_code": "missing_authorization_header",
  "user_message": "Sessao nao encontrada. Faca login para continuar."
}
```

Campos:

- `error`: alias de compatibilidade com clientes legados.
- `error_code`: codigo estavel para logica de frontend.
- `user_message`: mensagem amigavel para exibicao ao usuario final.

## 3) Compatibilidade

O campo `error` foi mantido para nao quebrar integrações antigas.

Clientes novos devem priorizar:

1. `error_code` para logica de tratamento.
2. `user_message` para UI.

## 4) Regras de mapeamento

- Se o erro ja vier em formato snake_case (ex.: `forbidden_for_ad_account`), ele e mantido.
- Se vier texto nao padronizado (ex.: `"failed to list clients"`), ele e normalizado para codigo estavel (ex.: `failed_to_list_clients`).
- Se nao for possivel normalizar, aplica fallback por status HTTP:
  - `400` -> `bad_request`
  - `401` -> `unauthorized`
  - `403` -> `forbidden`
  - `404` -> `not_found`
  - `>=500` -> `internal_error`

## 5) Exemplos por cenario

### 5.1 Nao autenticado

`401 Unauthorized`

```json
{
  "error": "missing_authorization_header",
  "error_code": "missing_authorization_header",
  "user_message": "Sessao nao encontrada. Faca login para continuar."
}
```

### 5.2 Sem permissao na conta

`403 Forbidden`

```json
{
  "error": "insufficient_role_for_ad_account",
  "error_code": "insufficient_role_for_ad_account",
  "user_message": "Voce nao tem permissao para executar esta acao nesta conta de anuncios."
}
```

### 5.3 Erro de validacao de payload

`400 Bad Request`

```json
{
  "error": "missing_ad_account_id",
  "error_code": "missing_ad_account_id",
  "user_message": "Selecione uma conta de anuncios para continuar."
}
```

### 5.4 Falha interna

`500 Internal Server Error`

```json
{
  "error": "internal_error",
  "error_code": "internal_error",
  "user_message": "Ocorreu um erro interno. Tente novamente em instantes."
}
```

## 6) Logging tecnico no backend

Detalhe tecnico do erro nao e retornado no payload para o cliente.

O backend registra em log:

- status HTTP;
- `error_code`;
- mensagem/causa original (`cause`) quando aplicavel.

## 7) Arquivos envolvidos

- `internal/httpapi/responses.go`
- `internal/httpapi/handlers.go`

## 8) Uso recomendado no frontend

Ordem de prioridade:

1. Exibir `user_message`.
2. Registrar `error_code` e detalhes tecnicos no console/telemetria.
3. Usar `error` apenas como fallback de compatibilidade.
