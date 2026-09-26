declare namespace API {
  type AccountDeletionReceiptResponse = {
    access_closed: boolean
    completed_at?: string
    generation_count: number
    id: string
    media_count: number
    phase:
      | 'media_cleanup'
      | 'media_cleanup_retry_observed'
      | 'generation_cleanup'
      | 'complete'
    receipt_expires_at: string
    remaining_generation_count: number
    remaining_media_count: number
    requested_at: string
    retry_observed: boolean
    status: 'pending' | 'complete'
    updated_at: string
  }

  type AccountDeletionResponse = {
    completed_at?: string
    generation_count: number
    id: string
    media_count: number
    receipt_expires_at: string
    /** 仅在注销 202 响应交付一次；请安全保存 */
    receipt_token: string
    requested_at: string
    status: 'pending' | 'complete'
  }

  type AdminAppealPageResponse = {
    appeals: any
    next_after_id?: string
  }

  type AdminAppealResponse = {
    action: string
    action_id: string
    appellant_id: string
    created_at: string
    id: string
    reason: string
    resolution_code?: string
    resolved_at?: string
    status: 'open' | 'upheld' | 'reversed'
  }

  type AdminReportPageResponse = {
    next_after_id?: string
    reports: any
  }

  type AdminReportResponse = {
    comment_id?: string
    created_at: string
    detail?: string
    id: string
    post_id: string
    reason_code: string
    reporter_id: string
    resolution_code?: string
    resolved_at?: string
    status: 'open' | 'resolved' | 'dismissed'
    target_type: 'post' | 'comment'
  }

  type AdminUserPageResponse = {
    next_after_id?: string
    users: any
  }

  type AdminUserResponse = {
    created_at: string
    display_name: string
    id: string
    revision: number
    role: 'user' | 'moderator' | 'admin'
    status: 'active' | 'suspended' | 'deleting'
    updated_at: string
  }

  type AppealPageResponse = {
    appeals: any
    next_after_id?: string
  }

  type AppealResponse = {
    action: string
    action_id: string
    created_at: string
    id: string
    reason: string
    resolution_code?: string
    resolved_at?: string
    status: 'open' | 'upheld' | 'reversed'
  }

  type archiveWardrobeItemParams = {
    item_id: string
  }

  type AuthenticatedUserResponse = {
    user: UserResponse
  }

  type BlockedUserPageResponse = {
    next_after_id?: string
    users: any
  }

  type BlockedUserResponse = {
    blocked_at: string
    display_name: string
    handle: string
    user_id: string
  }

  type blockUserParams = {
    user_id: string
  }

  type bookmarkPostParams = {
    post_id: string
  }

  type CalendarDayResponse = {
    diary_count: number
    local_date: string
    plan_count: number
    wear_event_count: number
  }

  type CalendarMonthResponse = {
    days: any
    month: string
  }

  type cancelGenerationJobParams = {
    job_id: string
  }

  type cancelOutfitPlanParams = {
    plan_id: string
  }

  type CancelOutfitPlanRequest = {
    expected_revision: number
  }

  type CommentModerationPageResponse = {
    comments: any
    next_after_id?: string
  }

  type CommentModerationResponse = {
    author_display_name: string
    author_handle: string
    body?: string
    created_at: string
    id: string
    parent_id?: string
    post_author_handle: string
    post_id: string
    post_title?: string
    reason_code?: string
    revision: number
    state: 'pending' | 'published' | 'rejected' | 'deleted' | 'removed'
    updated_at: string
  }

  type CommentPageResponse = {
    comments: any
    next_after_id?: string
  }

  type CommentResponse = {
    author_display_name: string
    author_handle: string
    body?: string
    created_at: string
    id: string
    parent_id?: string
    post_id: string
    reason_code?: string
    revision: number
    state: 'pending' | 'published' | 'rejected' | 'deleted' | 'removed'
    updated_at: string
  }

  type completeMediaUploadParams = {
    media_id: string
  }

  type CompleteMediaUploadRequest = {
    version_id: string
  }

  type ConfirmMailChallengeInputBody = {
    token: string
  }

  type ConfirmPasswordResetInputBody = {
    /** 12–72 个 UTF-8 字节 */
    new_password: string
    token: string
  }

  type ConfirmSelfAdultDeclarationRequest = {
    /** 用户主动确认照片仅属于本人且已年满 18 周岁；必须为 true */
    confirms_self_and_adult: true
    /** 服务端当前本人成年声明版本 */
    policy_version: 'self-adult-v1'
  }

  type ConsentResponse = {
    agreed_at: string
    category: 'person_photo' | 'ordinary_image'
    id: string
    max_retention_hours: 24
    policy_version: 'person-photo-v1' | 'generation-input-v1'
    processor: 'then'
    purpose: 'avatar_source_preparation' | 'generation_input'
    region: 'local-development'
    status: 'active' | 'withdrawn'
    training_allowed: false
    withdrawn_at?: string
  }

  type ContentReportResponse = {
    comment_id?: string
    created_at: string
    detail?: string
    id: string
    post_id: string
    reason_code: string
    resolution_code?: string
    resolved_at?: string
    status: 'open' | 'resolved' | 'dismissed'
    target_type: 'post' | 'comment'
  }

  type CreateAppealRequest = {
    action_id: string
    id: string
    reason: string
  }

  type createCommentParams = {
    post_id: string
  }

  type CreateCommentRequest = {
    body: string
    id: string
    parent_id?: string
  }

  type CreateConsentRequest = {
    actively_agreed: true
    category: 'person_photo' | 'ordinary_image'
    max_retention_hours: 24
    policy_version: 'person-photo-v1' | 'generation-input-v1'
    processor: 'then'
    purpose: 'avatar_source_preparation' | 'generation_input'
    region: 'local-development'
    training_allowed: false
  }

  type CreateDataExportInputBody = {
    mode: 'structured' | 'with_media'
    password: string
  }

  type CreateDiaryEntryRequest = {
    body?: string
    id: string
    local_date: string
    media_ids: any
    mood?: string
    occasion?: string
    plan_id?: string
    time_zone: string
    title?: string
    wear_event_id?: string
  }

  type CreateGenerationJobRequest = {
    consent: GenerationConsentRequest
    idempotency_key: string
    image_asset_id?: string
    image_sha256?: string
    inputs: any
    look_id: string
    look_revision: number
    model: string
    parameters?: Record<string, any>
    provider: string
    purpose: 'image' | 'model'
  }

  type CreateMediaUploadRequest = {
    byte_size: number
    category?: 'person_photo' | 'ordinary_image'
    consent_id?: string
    content_type: 'image/jpeg'
    purpose:
      | 'avatar_source_preparation'
      | 'generation_input'
      | 'diary_image'
      | 'community_publish'
      | 'profile_avatar'
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

  type CreatePostRequest = {
    body?: string
    id: string
    media_ids: any
    source_diary_id?: string
    tags: any
    title?: string
  }

  type CreateReportRequest = {
    comment_id?: string
    detail?: string
    id: string
    post_id: string
    reason_code:
      | 'spam'
      | 'harassment'
      | 'sexual'
      | 'violence'
      | 'misinformation'
      | 'other'
    target_type: 'post' | 'comment'
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

  type DataExportResponse = {
    completed_at?: string
    counts: Record<string, any>
    created_at: string
    expires_at: string
    id: string
    mode: 'structured' | 'with_media'
    omissions: any
    status: 'preparing' | 'ready' | 'partial' | 'failed' | 'expired' | 'revoked'
  }

  type decideCommentModerationParams = {
    comment_id: string
  }

  type DecideCommentRequest = {
    decision: 'approve' | 'reject'
    expected_revision: number
    reason_code: string
  }

  type decidePostModerationParams = {
    post_id: string
  }

  type DecidePostRequest = {
    decision: 'approve' | 'reject'
    expected_revision: number
    reason_code: string
    review_round: number
    version: number
  }

  type deleteCommentParams = {
    comment_id: string
    expected_revision?: number
  }

  type deleteDiaryEntryParams = {
    entry_id: string
    expected_revision?: number
  }

  type deleteGenerationJobParams = {
    job_id: string
  }

  type deleteMediaParams = {
    media_id: string
  }

  type deleteOutfitPlanParams = {
    plan_id: string
    expected_revision?: number
  }

  type deletePostParams = {
    post_id: string
    expected_revision?: number
  }

  type deleteProfileAvatarParams = {
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

  type deleteWearFeedbackParams = {
    wear_event_id: string
    feedback_id?: string
    mutation_id?: string
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

  type DiaryDeletionImpactResponse = {
    entry_id: string
    media_count: number
    media_retained: boolean
    published_post_count: number
    revision: number
  }

  type DiaryEntryPageResponse = {
    entries: any
    next_after_id?: string
  }

  type DiaryEntryResponse = {
    body?: string
    created_at: string
    id: string
    local_date: string
    media_ids: any
    mood?: string
    occasion?: string
    plan_id?: string
    revision: number
    time_zone: string
    title?: string
    updated_at: string
    wear_event_id?: string
  }

  type downloadDataExportParams = {
    id: string
  }

  type ErrorResponse = {
    code:
      | 'BAD_REQUEST'
      | 'EMAIL_CONFLICT'
      | 'CONFLICT'
      | 'EXPORT_NOT_READY'
      | 'PAYLOAD_TOO_LARGE'
      | 'AUTHENTICATION_FAILED'
      | 'FORBIDDEN'
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

  type FeedbackResponse = {
    activity_comfort?: 'uncomfortable' | 'okay' | 'comfortable'
    created_at: string
    id: string
    issue_tags: any
    note?: string
    occasion_fit?: 'tooCasual' | 'right' | 'tooFormal'
    repeat_intent?: 'yes' | 'unsure' | 'no'
    revision: number
    thermal_comfort?: 'cold' | 'comfortable' | 'hot'
    updated_at: string
    wear_event_id: string
  }

  type followProfileParams = {
    handle: string
  }

  type GenerationCleanupResponse = {
    access_revoked_at: string
    attempts: number
    completed_at?: string
    id: string
    next_attempt_at?: string
    status: 'pending' | 'running' | 'complete' | 'failed'
  }

  type GenerationConsentRequest = {
    accepted_at: string
    id: string
    policy_version: string
    purpose: 'image' | 'model'
  }

  type GenerationInputReferenceRequest = {
    media_id: string
    ordinal: number
    revision: number
    role: 'person' | 'garment' | 'look_image'
    sha256: string
  }

  type GenerationJobPageResponse = {
    jobs: any
    next_after_id?: string
  }

  type GenerationJobResponse = {
    access_revoked_at?: string
    cancel_requested_at?: string
    cleanup?: GenerationCleanupResponse
    created_at: string
    external_task_id?: string
    failure_code?: string
    id: string
    look_id: string
    look_revision: number
    match?: 'none' | 'idempotent_replay' | 'content_dedupe'
    model: string
    output?: GenerationOutputResponse
    provider: string
    purpose: 'image' | 'model'
    reservation?: GenerationReservationResponse
    result_asset_id?: string
    reused?: boolean
    status:
      | 'queued'
      | 'running'
      | 'validating'
      | 'succeeded'
      | 'failed'
      | 'canceled'
      | 'expired'
    status_revision: number
    submission_attempt: number
    submission_state: 'not_started' | 'in_flight' | 'unknown' | 'accepted'
    updated_at: string
  }

  type GenerationOutputAccessResponse = {
    byte_size: number
    content_type: 'image/jpeg' | 'model/gltf-binary'
    expires_at: string
    sha256: string
    url: string
  }

  type GenerationOutputResponse = {
    byte_size: number
    content_type: 'image/jpeg' | 'model/gltf-binary'
    id: string
    object_version_id: string
    published_at: string
    sha256: string
  }

  type GenerationReservationResponse = {
    created_at: string
    currency?: string
    estimated_minor_units: number
    id: string
    reserved_quota_units: number
    state: 'reserved' | 'released' | 'consumed'
    state_revision: number
    updated_at: string
  }

  type getAccountDeletionReceiptParams = {
    id: string
  }

  type getCalendarMonthParams = {
    month?: string
  }

  type getConsentParams = {
    consent_id: string
  }

  type getDataExportParams = {
    id: string
  }

  type getDeletionRequestParams = {
    request_id: string
  }

  type getDiaryEntryDeletionImpactParams = {
    entry_id: string
  }

  type getDiaryEntryParams = {
    entry_id: string
  }

  type getGenerationJobParams = {
    job_id: string
  }

  type getGenerationOutputAccessParams = {
    job_id: string
  }

  type getMediaParams = {
    media_id: string
  }

  type getOutfitPlanParams = {
    plan_id: string
  }

  type getOwnPostParams = {
    post_id: string
  }

  type getPostModerationCandidateParams = {
    post_id: string
    version: number
  }

  type getPostModerationImageParams = {
    post_id: string
    version: number
    ordinal: number
  }

  type getPublicPostImageParams = {
    post_id: string
    ordinal: number
  }

  type getPublicPostParams = {
    post_id: string
  }

  type getPublicProfileAvatarParams = {
    /** 公开主页唯一标识 */
    handle: string
  }

  type getPublicProfileParams = {
    /** 公开主页唯一标识 */
    handle: string
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

  type getWearFeedbackParams = {
    wear_event_id: string
  }

  type getWearStatisticsParams = {
    from?: string
    to?: string
  }

  type likePostParams = {
    post_id: string
  }

  type listAdminUsersParams = {
    limit?: number
    after_id?: string
  }

  type listBlockedUsersParams = {
    limit?: number
    after_id?: string
  }

  type listBookmarksParams = {
    limit?: number
    after_id?: string
  }

  type listCommentModerationCandidatesParams = {
    limit?: number
    after_id?: string
  }

  type listCommentRepliesParams = {
    limit?: number
    after_id?: string
    comment_id: string
  }

  type listCommentsParams = {
    limit?: number
    after_id?: string
    post_id: string
    parent_id?: string
  }

  type listCommunityFeedParams = {
    limit?: number
    after_id?: string
    type?: 'discover' | 'following'
  }

  type listCommunityReportsParams = {
    limit?: number
    after_id?: string
    status?: 'open' | 'resolved' | 'dismissed'
  }

  type listCurrentUserSessionsParams = {
    limit?: number
    offset?: number
  }

  type listDiaryEntriesParams = {
    limit?: number
    after_id?: string
    date_from?: string
    date_to?: string
  }

  type listGenerationJobsParams = {
    limit?: number
    after_id?: string
  }

  type listModerationActionsParams = {
    limit?: number
    after_id?: string
  }

  type listModerationAppealsParams = {
    limit?: number
    after_id?: string
    status?: 'open' | 'upheld' | 'reversed'
  }

  type listNotificationsParams = {
    limit?: number
    after_id?: string
  }

  type listOutfitPlansParams = {
    limit?: number
    after_id?: string
    local_date?: string
  }

  type listOwnModerationAppealsParams = {
    limit?: number
    after_id?: string
  }

  type listOwnPostsParams = {
    limit?: number
    after_id?: string
    state?: 'draft' | 'pending' | 'published' | 'withdrawn' | 'removed'
  }

  type listOwnReportsParams = {
    limit?: number
    after_id?: string
    status?: 'open' | 'resolved' | 'dismissed'
  }

  type listPostModerationCandidatesParams = {
    limit?: number
    after_id?: string
  }

  type listProfileFollowersParams = {
    limit?: number
    after_id?: string
    handle: string
  }

  type listProfileFollowingParams = {
    limit?: number
    after_id?: string
    handle: string
  }

  type listProfilePostsParams = {
    limit?: number
    after_id?: string
    handle: string
  }

  type listSyncChangesParams = {
    after?: string
    limit?: number
  }

  type listUnknownGenerationSubmissionsParams = {
    limit?: number
    after_id?: string
  }

  type listWardrobeItemsParams = {
    limit?: number
    after_id?: string
    lifecycle?: 'active' | 'archived' | 'all'
    availability?: 'wearable' | 'laundry' | 'lent_out' | 'packed'
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

  type markNotificationReadParams = {
    notification_id: string
  }

  type markOutfitPlanNotWornParams = {
    plan_id: string
  }

  type MediaResponse = {
    byte_size: number
    category: 'person_photo' | 'ordinary_image'
    consent_id?: string
    content_type: 'image/jpeg'
    created_at: string
    id: string
    pixel_height?: number
    pixel_width?: number
    purpose:
      | 'avatar_source_preparation'
      | 'generation_input'
      | 'diary_image'
      | 'community_publish'
      | 'profile_avatar'
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

  type ModerationActionPageResponse = {
    actions: any
    next_after_id?: string
  }

  type ModerationActionResponse = {
    action: string
    actor_id: string
    comment_id?: string
    created_at: string
    id: string
    post_id?: string
    post_version?: number
    reason_code: string
    report_id?: string
    subject_user_id?: string
  }

  type ModerationCandidatePageResponse = {
    candidates: any
    next_after_id?: string
  }

  type ModerationCandidateResponse = {
    author_display_name: string
    author_handle: string
    body?: string
    image_count: number
    post_id: string
    post_revision: number
    review_round: number
    submitted_at: string
    tags: any
    title?: string
    version: number
  }

  type NotificationPageResponse = {
    next_after_id?: string
    notifications: any
  }

  type NotificationResponse = {
    actor_handle?: string
    comment_id?: string
    created_at: string
    id: string
    kind:
      | 'comment_published'
      | 'reply_published'
      | 'followed'
      | 'post_reviewed'
      | 'comment_reviewed'
      | 'appeal_resolved'
    post_id?: string
    read_at?: string
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

  type PostPageResponse = {
    next_after_id?: string
    posts: any
  }

  type PostResponse = {
    created_at: string
    current: PostRevisionResponse
    draft_version?: number
    id: string
    pending_version?: number
    published?: PostRevisionResponse
    published_at?: string
    published_version?: number
    review_round: number
    revision: number
    source_diary_id?: string
    state: 'draft' | 'pending' | 'published' | 'withdrawn' | 'removed'
    updated_at: string
    withdrawn_at?: string
  }

  type PostRevisionRequest = {
    expected_revision: number
  }

  type PostRevisionResponse = {
    body?: string
    created_at: string
    media_ids: any
    reason_code?: string
    review_round: number
    review_state: 'draft' | 'pending' | 'approved' | 'rejected' | 'cancelled'
    reviewed_at?: string
    submitted_at?: string
    tags: any
    title?: string
    version: number
  }

  type PublicPostPageResponse = {
    next_after_id?: string
    posts: any
  }

  type PublicPostResponse = {
    author_display_name: string
    author_handle: string
    body?: string
    comment_count: number
    following_author: boolean
    id: string
    image_count: number
    like_count: number
    published_at: string
    tags: any
    title?: string
    viewer_bookmarked: boolean
    viewer_liked: boolean
  }

  type PublicProfilePageResponse = {
    next_after_id?: string
    profiles: any
  }

  type PublicProfileResponse = {
    /** 当前有头像时指向同源净化图；无头像为 null */
    avatar_url: any
    bio?: string
    created_at: string
    display_name: string
    handle: string
    revision: number
    updated_at: string
  }

  type PublicProfileSummaryResponse = {
    bio?: string
    display_name: string
    follower_count: number
    following: boolean
    following_count: number
    handle: string
  }

  type PutProfileAvatarRequest = {
    expected_revision: number
    media_id: string
  }

  type PutProfileRequest = {
    /** 公开简介；空白值保存为未设置 */
    bio?: string
    /** 首次创建传 0；后续修改传上次读取到的版本 */
    expected_revision: number
    /** 公开主页唯一标识；保存时转为小写 */
    handle: string
  }

  type ReadinessResponse = {
    request_id: string
    status: 'ready'
  }

  type reconcileUnknownGenerationSubmissionParams = {
    job_id: string
  }

  type ReconcileUnknownSubmissionRequest = {
    decision: 'accepted' | 'not_accepted'
    evidence_reference: string
    evidence_type: 'provider_console' | 'provider_query' | 'support_case'
    expected_revision: number
    external_task_id?: string
  }

  type RegisterAccountRequest = {
    /** 用户显示名称 */
    display_name: string
    /** 登录邮箱 */
    email: string
    /** 服务端要求 12–72 个 UTF-8 字节 */
    password: string
  }

  type RemovePostRequest = {
    expected_revision: number
    reason_code: string
  }

  type removePublishedCommentParams = {
    comment_id: string
  }

  type removePublishedPostParams = {
    post_id: string
  }

  type ReportPageResponse = {
    next_after_id?: string
    reports: any
  }

  type RequestPasswordResetInputBody = {
    email: string
  }

  type ResolveAppealRequest = {
    resolution_code: string
    status: 'upheld' | 'reversed'
  }

  type resolveCommunityReportParams = {
    report_id: string
  }

  type resolveModerationAppealParams = {
    appeal_id: string
  }

  type ResolveReportRequest = {
    resolution_code: string
    status: 'resolved' | 'dismissed'
  }

  type restoreOutfitPlanParams = {
    plan_id: string
  }

  type restoreUserParams = {
    user_id: string
  }

  type restoreWardrobeItemParams = {
    item_id: string
  }

  type revokeAccountDeletionReceiptParams = {
    id: string
  }

  type revokeCurrentUserSessionParams = {
    session_id: string
  }

  type revokeDataExportParams = {
    id: string
  }

  type SaveFeedbackRequest = {
    activity_comfort?: 'uncomfortable' | 'okay' | 'comfortable'
    expected_revision?: number
    feedback_id: string
    issue_tags: any
    mutation_id: string
    note?: string
    occasion_fit?: 'tooCasual' | 'right' | 'tooFormal'
    repeat_intent?: 'yes' | 'unsure' | 'no'
    thermal_comfort?: 'cold' | 'comfortable' | 'hot'
  }

  type saveWearFeedbackParams = {
    wear_event_id: string
  }

  type searchCommunityPostsParams = {
    limit?: number
    after_id?: string
    q?: string
    tag?: string
  }

  type SelfAdultDeclarationResponse = {
    confirmed: boolean
    confirmed_at?: string
    policy_version: 'self-adult-v1'
    withdrawn_at?: string
  }

  type SessionPageResponse = {
    items: any
    next_offset: number
  }

  type SessionResponse = {
    created_at: string
    current: boolean
    expires_at: string
    id: string
  }

  type SetUserStatusRequest = {
    expected_revision: number
    reason_code: string
  }

  type SubmissionReconciliationResponse = {
    audit_id: string
    decision: 'accepted' | 'not_accepted'
    job: GenerationJobResponse
    recorded_at: string
  }

  type submitPostParams = {
    post_id: string
  }

  type suspendUserParams = {
    user_id: string
  }

  type SyncChangeResponse = {
    action: 'upsert' | 'delete'
    entity_id: string
    kind:
      | 'wardrobe_item'
      | 'outfit_plan'
      | 'wear_event'
      | 'wear_feedback'
      | 'diary_entry'
    revision: number
    seq: number
  }

  type SyncChangesResponse = {
    changes: any
    has_more: boolean
    next_cursor: string
  }

  type TransitionOutfitPlanRequest = {
    expected_revision: number
  }

  type unblockUserParams = {
    user_id: string
  }

  type unbookmarkPostParams = {
    post_id: string
  }

  type unfollowProfileParams = {
    handle: string
  }

  type UnknownSubmissionPageResponse = {
    jobs: any
    next_after_id?: string
  }

  type UnknownSubmissionResponse = {
    access_revoked_at?: string
    cancel_requested_at?: string
    created_at: string
    id: string
    model: string
    provider: string
    purpose: 'image' | 'model'
    status_revision: number
    submission_attempt: number
    unknown_at: string
    updated_at: string
  }

  type unlikePostParams = {
    post_id: string
  }

  type updateDiaryEntryParams = {
    entry_id: string
  }

  type UpdateDiaryEntryRequest = {
    body?: string
    expected_revision: number
    local_date: string
    media_ids: any
    mood?: string
    occasion?: string
    plan_id?: string
    time_zone: string
    title?: string
    wear_event_id?: string
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

  type updatePostParams = {
    post_id: string
  }

  type UpdatePostRequest = {
    body?: string
    expected_revision: number
    media_ids: any
    source_diary_id?: string
    tags: any
    title?: string
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
    /** 当前登录邮箱已完成验证 */
    email_verified: boolean
    id: string
    revision: number
    role: 'user' | 'moderator' | 'admin'
    status: 'active' | 'suspended' | 'deleting'
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
    archived_at?: string
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
    lifecycle: 'active' | 'archived'
    name: string
    revision: number
    source: 'wardrobe' | 'quick_add'
    updated_at: string
  }

  type WardrobeLifecycleRequest = {
    expected_revision: number
  }

  type WardrobePageResponse = {
    items: any
    next_after_id?: string
  }

  type WardrobeRecommendationCandidateResponse = {
    items: any
    reasons: any
    uncertainties: any
  }

  type WardrobeRecommendationRequest = {
    formality_band?: 'casual' | 'smart_casual' | 'formal'
    include_packed_items: boolean
    local_date: string
    requires_rain_suitability: boolean
    requires_walking_suitability: boolean
    time_zone: string
    warmth_band?: 'light' | 'medium' | 'warm'
  }

  type WardrobeRecommendationResponse = {
    candidates: any
    gap?: string
    policy_version: string
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

  type WearItemUseResponse = {
    count: number
    item_id: string
  }

  type WearStatisticsResponse = {
    from: string
    item_uses: any
    to: string
    wear_days: number
    wear_events: number
  }

  type withdrawConsentParams = {
    consent_id: string
  }

  type withdrawPostParams = {
    post_id: string
  }
}
