package i18n

var ja = map[MsgKey]string{
	MsgTicketCreated:     "<{url}|チケット #{id}> を作成しました",
	MsgStatusChange:      "ステータス: *{old}* → *{new}*",
	MsgStatusChangeLabel: "ステータス",

	MsgTriageProgressHeader:  "triageを進めています: {message}",
	MsgTriageProgressQueued:  "⏳ {request} — 待機中",
	MsgTriageProgressRunning: "🔄 {request} — {trace}",
	MsgTriageProgressDone:    "✅ {request} — 完了",
	MsgTriageProgressFailed:  "❌ {request} — 失敗: {error}",

	MsgTriageAskHeader:          "triageに必要な情報をご記入ください:\n_{message}_",
	MsgTriageAskSubmitButton:    "送信",
	MsgTriageAskOtherTextLabel:  "上記に当てはまらない場合・補足",
	MsgTriageAskReceived:        "回答を受け取りました。ありがとうございます。",
	MsgTriageAskInvalidated:     "このフォームは無効になりました。",
	MsgTriageAskValidationError: "選択肢を1つ以上選ぶか、自由入力欄に内容を記入してください。",
	MsgTriageAskHistoryMissing:  "フォームの情報が見つからないため、この回答を受け付けられません。",

	MsgTriageCompleteHeaderAssigned:   "triageが完了しました",
	MsgTriageCompleteHeaderUnassigned: "triageが完了しました（担当者未定）",
	MsgTriageCompleteAssigneeMention:  "<@{user}> 対応をお願いします。",
	MsgTriageCompleteSectionSummary:   "*サマリ*",
	MsgTriageCompleteSectionFindings:  "*重要事項*",
	MsgTriageCompleteSectionAnswers:   "*依頼者からの回答*",
	MsgTriageCompleteSectionSimilar:   "*類似チケット*",
	MsgTriageCompleteSectionNextSteps: "*推奨アクション*",
	MsgTriageCompleteUnassignedReason: "理由: {reason}",
	MsgTriageAbortedHeader:            "triageを完了できませんでした",
	MsgTriageAbortedReason:            "理由: {reason}",
}
