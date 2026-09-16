declare namespace API {
  type AuthenticatedUserResponse = {
    user: UserResponse
  }

  type cancelOutfitPlanParams = {
    plan_id: string
  }

  type CancelOutfitPlanRequest = {
    expected_revision: number
  }

  type completeMediaUploadParams = {
    media_id: string
  }

  type CompleteMediaUploadRequest = {
    version_id: string
  }

  type ConfirmSelfAdultDeclarationRequest = {
    /** 用户主动确认照片仅属于本人且已年满 18 周岁；必须为 true */
    confirms_self_and_adult: true
    /** 服务端当前本人成年声明版本 */
    policy_version: 'self-adult-v1'
  }

  type ConsentResponse = {
    agreed_at: string
    category: 'person_photo'
    id: string
    max_retention_hours: 24
    policy_version: 'person-photo-v1'
    processor: 'then'
    purpose: 'avatar_source_preparation'
    region: 'local-development'
    status: 'active' | 'withdrawn'
    training_allowed: false
    withdrawn_at?: string
  }

  type CreateConsentRequest = {
    actively_agreed: true
    category: 'person_photo'
    max_retention_hours: 24
    policy_version: 'person-photo-v1'
    processor: 'then'
    purpose: 'avatar_source_preparation'
    region: 'local-development'
    training_allowed: false
  }

  type CreateMediaUploadRequest = {
    byte_size: number
    consent_id: string
    content_type: 'image/jpeg'
    purpose: 'avatar_source_preparation'
    sha256: string
  }

  type CreateOutfitPlanRequest = {
    confirmed_unavailable_ids: any
    context_summary?: string
    id: string
    items: any
    local_date: string
    time_zone: string
  }

  type CreateSessionRequest = {
    /** 登录邮箱 */
    email: string
    /** 服务端按 UTF-8 字节执行密码规则；登录失败统一返回 401 */
    password: string
  }

  type CreateWardrobeItemRequest = {
    /** 用户明确确认的可选衣物属性；空对象表示全部未知 */
    attributes: WardrobeAttributesRequest
    availability: 'wearable' | 'laundry' | 'lent_out' | 'packed'
    category:
      | 'top'
      | 'bottom'
      | 'one_piece'
      | 'outerwear'
      | 'shoes'
      | 'bag'
      | 'accessory'
    /** 客户端生成的稳定衣物 ID */
    id: string
    /** 用户确认的衣物名称 */
    name: string
    source: 'wardrobe' | 'quick_add'
  }

  type CreateWearEventRequest = {
    completeness: 'partial' | 'complete'
    confirmed_unavailable_ids: any
    context_summary?: string
    duplicate_confirmations: any
    id: string
    items: any
    laundry_item_ids: any
    local_date: string
    source_kind:
      | 'followed_plan'
      | 'changed_plan'
      | 'different_outfit'
      | 'unplanned'
    source_plan_id?: string
    source_plan_revision?: number
    time_zone: string
  }

  type deleteMediaParams = {
    media_id: string
  }

  type deleteOutfitPlanParams = {
    plan_id: string
    expected_revision?: number
  }

  type deleteWardrobeItemParams = {
    item_id: string
    expected_revision?: number
    history_policy?: 'redact_snapshots' | 'delete_affected_history'
    expected_impact?: string
  }

  type deleteWearEventParams = {
    wear_event_id: string
    expected_revision?: number
  }

  type DeletionRequestResponse = {
    attempts: number
    backup_expires_at: string
    completed_at?: string
    created_at: string
    error?: string
    id: string
    media_id: string
    read_revoked_at: string
    status: 'pending' | 'running' | 'complete' | 'failed'
    updated_at: string
  }

  type ErrorResponse = {
    code:
      | 'BAD_REQUEST'
      | 'EMAIL_CONFLICT'
      | 'CONFLICT'
      | 'PAYLOAD_TOO_LARGE'
      | 'AUTHENTICATION_FAILED'
      | 'RATE_LIMITED'
      | 'NOT_READY'
      | 'NOT_FOUND'
      | 'METHOD_NOT_ALLOWED'
      | 'INTERNAL_ERROR'
    duplicate_candidates?: any
    message: string
    request_id: string
    retryable: boolean
  }

  type getConsentParams = {
    consent_id: string
  }

  type getDeletionRequestParams = {
    request_id: string
  }

  type getMediaParams = {
    media_id: string
  }

  type getOutfitPlanParams = {
    plan_id: string
  }

  type getWardrobeDeletionImpactParams = {
    item_id: string
  }

  type getWardrobeItemParams = {
    item_id: string
  }

  type getWearEventParams = {
    wear_event_id: string
  }

  type listOutfitPlansParams = {
    limit?: number
    after_id?: string
    local_date?: string
  }

  type listWardrobeItemsParams = {
    limit?: number
    after_id?: string
  }

  type listWearEventsParams = {
    limit?: number
    after_id?: string
    local_date?: string
  }

  type LivenessResponse = {
    request_id: string
    status: 'live'
  }

  type markOutfitPlanNotWornParams = {
    plan_id: string
  }

  type MediaResponse = {
    byte_size: number
    category: 'person_photo'
    consent_id: string
    content_type: 'image/jpeg'
    created_at: string
    id: string
    pixel_height?: number
    pixel_width?: number
    purpose: 'avatar_source_preparation'
    reason?: string
    status:
      | 'pending_upload'
      | 'uploaded'
      | 'checking'
      | 'ready'
      | 'rejected'
      | 'deleting'
      | 'deleted'
    updated_at: string
  }

  type MediaUploadResponse = {
    expires_at: string
    headers: Record<string, any>
    media: MediaResponse
    method: 'PUT'
    url: string
  }

  type OutfitPlanItemContentResponse = {
    attributes: WardrobeAttributesResponse
    availability: 'wearable' | 'laundry' | 'lent_out' | 'packed'
    category:
      | 'top'
      | 'bottom'
      | 'one_piece'
      | 'outerwear'
      | 'shoes'
      | 'bag'
      | 'accessory'
    item_id: string
    item_revision: number
    name: string
  }

  type OutfitPlanItemResponse = {
    /** NULL 表示衣物已按用户选择从历史快照清除 */
    content: OutfitPlanItemContentResponse
    ordinal: number
  }

  type OutfitPlanPageResponse = {
    next_after_id?: string
    plans: any
  }

  type OutfitPlanResponse = {
    context_summary?: string
    created_at: string
    id: string
    items: any
    local_date: string
    revision: number
    status: 'active' | 'completed' | 'not_worn' | 'cancelled'
    time_zone: string
    updated_at: string
  }

  type OutfitSelectionRequest = {
    item_id: string
    revision: number
  }

  type ReadinessResponse = {
    request_id: string
    status: 'ready'
  }

  type RegisterAccountRequest = {
    /** 用户显示名称 */
    display_name: string
    /** 登录邮箱 */
    email: string
    /** 服务端要求 12–72 个 UTF-8 字节 */
    password: string
  }

  type restoreOutfitPlanParams = {
    plan_id: string
  }

  type SelfAdultDeclarationResponse = {
    confirmed: boolean
    confirmed_at?: string
    policy_version: 'self-adult-v1'
    withdrawn_at?: string
  }

  type TransitionOutfitPlanRequest = {
    expected_revision: number
  }

  type updateOutfitPlanParams = {
    plan_id: string
  }

  type UpdateOutfitPlanRequest = {
    confirmed_unavailable_ids: any
    context_summary?: string
    expected_revision: number
    items: any
    local_date: string
    time_zone: string
  }

  type updateWardrobeItemParams = {
    item_id: string
  }

  type UpdateWardrobeItemRequest = {
    /** 完整替换的用户确认属性；NULL 表示清除为未知 */
    attributes: WardrobeAttributesRequest
    availability: 'wearable' | 'laundry' | 'lent_out' | 'packed'
    category:
      | 'top'
      | 'bottom'
      | 'one_piece'
      | 'outerwear'
      | 'shoes'
      | 'bag'
      | 'accessory'
    expected_revision: number
    name: string
  }

  type updateWearEventParams = {
    wear_event_id: string
  }

  type UpdateWearEventRequest = {
    completeness: 'partial' | 'complete'
    confirmed_unavailable_ids: any
    context_summary?: string
    duplicate_confirmations: any
    expected_revision: number
    items: any
    laundry_item_ids: any
    local_date: string
    source_kind:
      | 'followed_plan'
      | 'changed_plan'
      | 'different_outfit'
      | 'unplanned'
    source_plan_id?: string
    source_plan_revision?: number
    time_zone: string
  }

  type UserResponse = {
    created_at: string
    display_name: string
    email: string
    id: string
    updated_at: string
  }

  type WardrobeAttributesRequest = {
    /** 用户确认的正式度；省略或 NULL 表示未知 */
    formality_band?: 'casual' | 'smart_casual' | 'formal'
    /** 用户确认的雨天适用判断；省略或 NULL 表示未知 */
    rain_use?: 'suitable' | 'unsuitable'
    /** 用户确认的步行适用判断；省略或 NULL 表示未知 */
    walking_use?: 'suitable' | 'unsuitable'
    /** 用户确认的保暖感受；省略或 NULL 表示未知 */
    warmth_band?: 'light' | 'medium' | 'warm'
  }

  type WardrobeAttributesResponse = {
    formality_band: WardrobeFormalityAttributeResponse
    rain_use: WardrobeSuitabilityAttributeResponse
    walking_use: WardrobeSuitabilityAttributeResponse
    warmth_band: WardrobeWarmthAttributeResponse
  }

  type WardrobeDeletionImpactResponse = {
    affected_plan_count: number
    affected_wear_event_count: number
    /** 确认删除影响所需的不透明摘要 */
    expected_impact: string
  }

  type WardrobeFormalityAttributeResponse = {
    source: 'user_confirmed'
    value: 'casual' | 'smart_casual' | 'formal'
  }

  type WardrobeItemResponse = {
    attributes: WardrobeAttributesResponse
    availability: 'wearable' | 'laundry' | 'lent_out' | 'packed'
    category:
      | 'top'
      | 'bottom'
      | 'one_piece'
      | 'outerwear'
      | 'shoes'
      | 'bag'
      | 'accessory'
    created_at: string
    id: string
    name: string
    revision: number
    source: 'wardrobe' | 'quick_add'
    updated_at: string
  }

  type WardrobePageResponse = {
    items: any
    next_after_id?: string
  }

  type WardrobeSuitabilityAttributeResponse = {
    source: 'user_confirmed'
    value: 'suitable' | 'unsuitable'
  }

  type WardrobeWarmthAttributeResponse = {
    source: 'user_confirmed'
    value: 'light' | 'medium' | 'warm'
  }

  type WearEventCandidateResponse = {
    id: string
    revision: number
  }

  type WearEventPageResponse = {
    events: any
    next_after_id?: string
  }

  type WearEventResponse = {
    completeness: 'partial' | 'complete'
    context_summary?: string
    created_at: string
    id: string
    items: any
    local_date: string
    revision: number
    source_kind:
      | 'followed_plan'
      | 'changed_plan'
      | 'different_outfit'
      | 'unplanned'
    source_plan_id?: string
    source_plan_revision?: number
    time_zone: string
    updated_at: string
  }

  type withdrawConsentParams = {
    consent_id: string
  }
}
