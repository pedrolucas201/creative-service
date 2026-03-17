package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type errorResponse struct {
	Error       string `json:"error"`
	ErrorCode   string `json:"error_code"`
	UserMessage string `json:"user_message"`
}

var errorCodeAliases = map[string]string{
	"failed to list clients":     "failed_to_list_clients",
	"failed to list creatives":   "failed_to_list_creatives",
	"failed to list ad accounts": "failed_to_list_ad_accounts",
}

var errorUserMessages = map[string]string{
	"missing_authorization_header":                  "Sessão não encontrada. Faça login para continuar.",
	"invalid_authorization_header":                  "Não foi possível validar a sessão. Faça login novamente.",
	"missing_bearer_token":                          "Não foi possível validar a sessão. Faça login novamente.",
	"invalid_or_expired_token":                      "Sua sessão expirou. Faça login novamente.",
	"missing_identity":                              "Não foi possível identificar sua sessão. Faça login novamente.",
	"failed_to_sync_user":                           "Não foi possível validar seu usuário agora. Tente novamente.",
	"failed_to_check_access":                        "Não foi possível validar suas permissões agora. Tente novamente.",
	"forbidden_for_ad_account":                      "Você não tem acesso a esta conta de anúncios.",
	"insufficient_role_for_ad_account":              "Você não tem permissão para executar esta ação nesta conta de anúncios.",
	"missing_bm_uuid":                               "BM não informada.",
	"failed_to_check_bm_access":                     "Não foi possível validar acesso à BM agora. Tente novamente.",
	"forbidden_for_bm":                              "Você não tem acesso a esta BM.",
	"insufficient_role_for_bm":                      "Você não tem permissão para executar esta ação nesta BM.",
	"insufficient_role_for_bm_config":               "Você não tem permissão para visualizar esta configuração de BM.",
	"failed_to_get_bm_config":                       "Não foi possível carregar a configuração da BM.",
	"firebase_user_management_unavailable":          "Não foi possível acessar o gerenciador de usuários agora.",
	"missing_email":                                 "Informe um e-mail para continuar.",
	"invalid_email":                                 "O e-mail informado é inválido.",
	"invalid_role":                                  "A role informada é inválida.",
	"invalid_password_length":                       "O tamanho da senha deve ser 8 ou 10 caracteres.",
	"failed_to_generate_password":                   "Não foi possível gerar a senha temporária agora.",
	"failed_to_create_firebase_user":                "Não foi possível criar o usuário no Firebase agora.",
	"failed_to_bind_user_bm_access":                 "Não foi possível vincular o usuário à BM agora.",
	"invalid_multipart":                             "Arquivo ou formulário inválido. Revise e tente novamente.",
	"missing_ad_account_id":                         "Selecione uma conta de anúncios para continuar.",
	"missing_image":                                 "Selecione uma imagem para continuar.",
	"missing_video":                                 "Selecione um vídeo para continuar.",
	"missing_thumbnail":                             "Selecione uma miniatura para continuar.",
	"invalid_json":                                  "Os dados enviados estão inválidos.",
	"missing_name":                                  "Informe o nome para continuar.",
	"missing_objective":                             "Selecione o objetivo para continuar.",
	"missing_campaign_id":                           "Campanha não informada.",
	"missing_billing_event":                         "Billing event não informado.",
	"missing_optimization_goal":                     "Objetivo de otimização não informado.",
	"missing_daily_budget":                          "Informe o orçamento diário.",
	"missing_adset_id":                              "AdSet não informado.",
	"missing_creative_id":                           "Creative não informado.",
	"invalid_type_filter":                           "Filtro inválido.",
	"invalid_entity_type":                           "Tipo de entidade inválido. Use creative, campaign, adset ou ad.",
	"invalid_contingency_trigger_type":              "Tipo de gatilho inválido para contingência.",
	"invalid_contingency_status_filter":             "Filtro de status inválido. Use open ou all.",
	"invalid_contingency_close_status":              "Status final inválido para encerramento de contingência.",
	"missing_contingency_incident_uuid":             "Incidente de contingência não informado.",
	"invalid_limit":                                 "Limite inválido.",
	"creative_not_found":                            "Creative não encontrado.",
	"missing_client_uuid":                           "Cliente não informado.",
	"invalid_body":                                  "Corpo da requisição inválido.",
	"failed_to_list_clients":                        "Não foi possível listar os clientes agora.",
	"failed_to_list_creatives":                      "Não foi possível listar os criativos agora.",
	"failed_to_list_ad_accounts":                    "Não foi possível listar as contas de anúncios agora.",
	"failed_to_list_campaigns":                      "Não foi possível listar as campanhas agora.",
	"failed_to_list_adsets":                         "Não foi possível listar os adsets agora.",
	"failed_to_list_ads":                            "Não foi possível listar os anúncios agora.",
	"failed_to_create_image_creative":               "Não foi possível criar o criativo de imagem.",
	"failed_to_create_video_creative":               "Não foi possível criar o criativo de vídeo.",
	"failed_to_create_campaign":                     "Não foi possível criar a campanha.",
	"failed_to_create_adset":                        "Não foi possível criar o adset.",
	"failed_to_create_ad":                           "Não foi possível criar o anúncio.",
	"failed_to_sync_statuses":                       "Não foi possível sincronizar os status agora.",
	"failed_to_sync_webhook_status":                 "Não foi possível processar a atualização de status recebida.",
	"failed_to_list_statuses":                       "Não foi possível carregar os status agora.",
	"failed_to_fetch_meta_account_status":           "Não foi possível consultar o status da conta na Meta agora.",
	"failed_to_list_contingency_candidates":         "Não foi possível listar os candidatos de contingência agora.",
	"failed_to_list_contingency_incidents":          "Não foi possível listar os incidentes de contingência agora.",
	"failed_to_get_contingency_source_account":      "Não foi possível carregar a conta monitorada da contingência agora.",
	"failed_to_get_contingency_incident":            "Não foi possível carregar o incidente de contingência agora.",
	"failed_to_start_contingency_execution":         "Não foi possível iniciar a execução da contingência agora.",
	"failed_to_pick_contingency_target_node":        "Não foi possível selecionar o nó de contingência agora.",
	"failed_to_complete_contingency_execution":      "Não foi possível concluir a execução da contingência agora.",
	"failed_to_list_contingency_executions":         "Não foi possível listar as execuções da contingência agora.",
	"failed_to_list_contingency_switch_maps":        "Não foi possível listar os mapeamentos de switch da contingência agora.",
	"failed_to_list_contingency_available_accounts": "Não foi possível listar as contas elegíveis para contingência agora.",
	"failed_to_list_contingency_nodes":              "Não foi possível listar os nós de contingência agora.",
	"failed_to_list_contingency_routes":             "Não foi possível listar as rotas de contingência agora.",
	"failed_to_upsert_contingency_node":             "Não foi possível salvar o nó de contingência agora.",
	"failed_to_upsert_contingency_route":            "Não foi possível salvar a rota de contingência agora.",
	"failed_to_get_contingency_route":               "Não foi possível carregar a rota de contingência agora.",
	"failed_to_delete_contingency_route":            "Não foi possível remover a rota de contingência agora.",
	"failed_to_close_contingency_incident":          "Não foi possível encerrar o incidente de contingência agora.",
	"contingency_switch_failed":                     "Falha ao executar o switch da campanha para a conta de contingência.",
	"contingency_incident_not_found":                "Incidente de contingência não encontrado.",
	"contingency_route_not_found":                   "Rota de contingência não encontrada.",
	"contingency_incident_not_executable":           "Este incidente não está em um estado que permita execução.",
	"contingency_execution_in_progress":             "Já existe uma execução de contingência em andamento para este incidente.",
	"contingency_incident_already_closed":           "Este incidente já foi encerrado.",
	"contingency_incident_close_conflict":           "Não é possível encerrar este incidente enquanto uma execução estiver em andamento.",
	"no_target_node_for_contingency":                "Nenhum nó de contingência elegível foi encontrado.",
	"invalid_contingency_node":                      "Os dados do nó de contingência são inválidos para esta BM/conta.",
	"invalid_contingency_route":                     "Os dados da rota de contingência são inválidos para esta conta.",
	"invalid_contingency_cooldown":                  "A data de cooldown do nó está inválida. Use o formato ISO 8601.",
	"missing_contingency_target_node_uuid":          "Selecione um nó de destino para a rota.",
	"missing_contingency_route_uuid":                "Rota de contingência não informada.",
	"missing_contingency_ad_accounts":               "Nenhuma conta de anúncios foi informada para o monitor de contingência.",
	"contingency_internal_not_configured":           "Automação interna de contingência não configurada no servidor.",
	"invalid_internal_contingency_token":            "Token interno de contingência inválido.",
	"contingency_task_queue_not_configured":         "Fila de tarefas da contingência não está configurada no servidor.",
	"failed_to_delete_creative":                     "Não foi possível deletar o creative.",
	"failed_to_update_campaign":                     "Não foi possível atualizar a campanha.",
	"failed_to_delete_campaign":                     "Não foi possível deletar a campanha.",
	"failed_to_update_adset":                        "Não foi possível atualizar o adset.",
	"failed_to_delete_adset":                        "Não foi possível deletar o adset.",
	"failed_to_update_ad":                           "Não foi possível atualizar o anúncio.",
	"failed_to_delete_ad":                           "Não foi possível deletar o anúncio.",
	"cors_origin_not_allowed":                       "Origem não autorizada para esta API.",
	"meta_webhook_not_configured":                   "Webhook da Meta não configurado neste ambiente.",
	"invalid_webhook_verify_token":                  "Token de verificação do webhook inválido.",
	"invalid_webhook_signature":                     "Assinatura do webhook inválida.",
	"invalid_webhook_payload":                       "Payload do webhook inválido.",
	"internal_error":                                "Ocorreu um erro interno. Tente novamente em instantes.",
	"bad_request":                                   "Não foi possível processar os dados enviados.",
	"unauthorized":                                  "Sua sessão expirou. Faça login novamente.",
	"forbidden":                                     "Você não tem permissão para executar esta ação.",
	"not_found":                                     "Recurso não encontrado.",
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeErrInternal(w, status, msg, nil)
}

func writeErrCause(w http.ResponseWriter, status int, code string, cause error) {
	writeErrInternal(w, status, code, cause)
}

func writeErrInternal(w http.ResponseWriter, status int, msg string, cause error) {
	code := normalizeErrorCode(msg, status)
	userMessage := userMessageFor(code, status)

	if cause != nil {
		log.Printf("api_error status=%d code=%s msg=%q cause=%v", status, code, msg, cause)
	} else if code != strings.TrimSpace(msg) {
		log.Printf("api_error status=%d code=%s raw_msg=%q", status, code, msg)
	}

	writeJSON(w, status, errorResponse{
		Error:       code,
		ErrorCode:   code,
		UserMessage: userMessage,
	})
}

func normalizeErrorCode(msg string, status int) string {
	raw := strings.TrimSpace(strings.ToLower(msg))
	if raw == "" {
		return defaultCodeByStatus(status)
	}

	if alias, ok := errorCodeAliases[raw]; ok {
		return alias
	}

	if isSnakeCode(raw) {
		return raw
	}

	return defaultCodeByStatus(status)
}

func defaultCodeByStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	default:
		if status >= http.StatusInternalServerError {
			return "internal_error"
		}
		return "request_failed"
	}
}

func userMessageFor(code string, status int) string {
	if msg, ok := errorUserMessages[code]; ok {
		return msg
	}

	if status >= http.StatusInternalServerError {
		return "Houve uma falha temporária no servidor. Tente novamente em instantes."
	}
	return "Não foi possível concluir sua solicitação agora. Tente novamente."
}

func isSnakeCode(v string) bool {
	for _, r := range v {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}
