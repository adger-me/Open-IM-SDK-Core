package conversation_msg

import (
	"errors"
	"open_im_sdk/open_im_sdk_callback"
	"open_im_sdk/pkg/common"
	"open_im_sdk/pkg/constant"
	"open_im_sdk/pkg/db"
	"open_im_sdk/pkg/log"
	sdk "open_im_sdk/pkg/sdk_params_callback"
	"open_im_sdk/pkg/server_api_params"
	"open_im_sdk/pkg/utils"
	"open_im_sdk/sdk_struct"
)

func (c *Conversation) getAllConversationList(callback open_im_sdk_callback.Base, operationID string) sdk.GetAllConversationListCallback {
	conversationList, err := c.db.GetAllConversationList()
	common.CheckDBErrCallback(callback, err, operationID)
	return conversationList
}
func (c *Conversation) getConversationListSplit(callback open_im_sdk_callback.Base, offset, count int, operationID string) sdk.GetConversationListSplitCallback {
	conversationList, err := c.db.GetConversationListSplit(offset, count)
	common.CheckDBErrCallback(callback, err, operationID)
	return conversationList
}

func (c *Conversation) setConversationRecvMessageOpt(callback open_im_sdk_callback.Base, conversationIDList []string, opt int, operationID string) []*server_api_params.OptResult {
	apiReq := server_api_params.SetReceiveMessageOptReq{}
	apiReq.OperationID = operationID
	apiReq.FromUserID = c.loginUserID
	var temp int32
	temp = int32(opt)
	apiReq.Opt = &temp
	apiReq.ConversationIDList = conversationIDList
	var realData []*server_api_params.OptResult
	c.p.PostFatalCallback(callback, constant.SetReceiveMessageOptRouter, apiReq, realData, apiReq.OperationID)
	err := c.db.SetMultipleConversationRecvMsgOpt(conversationIDList, opt)
	if err != nil {
		log.Error(operationID, "SetMultipleConversationRecvMsgOpt err:", err.Error())
	}
	return realData
}
func (c *Conversation) getConversationRecvMessageOpt(callback open_im_sdk_callback.Base, conversationIDList []string, operationID string) []*server_api_params.OptResult {
	apiReq := server_api_params.GetReceiveMessageOptReq{}
	apiReq.OperationID = operationID
	apiReq.FromUserID = c.loginUserID
	apiReq.ConversationIDList = conversationIDList
	var realData []*server_api_params.OptResult
	c.p.PostFatalCallback(callback, constant.GetReceiveMessageOptRouter, apiReq, realData, apiReq.OperationID)
	return realData
}
func (c *Conversation) getOneConversation(callback open_im_sdk_callback.Base, sourceID string, sessionType int32, operationID string) *db.LocalConversation {
	conversationID := c.GetConversationIDBySessionType(sourceID, sessionType)
	lc, err := c.db.GetConversation(conversationID)
	common.CheckDBErrCallback(callback, err, operationID)
	if err == nil {
		return lc
	} else {
		var newConversation db.LocalConversation
		newConversation.ConversationID = conversationID
		newConversation.ConversationType = sessionType
		switch sessionType {
		case constant.SingleChatType:
			newConversation.UserID = sourceID
			faceUrl, name, err := c.getUserNameAndFaceUrlByUid(callback, sourceID, operationID)
			common.CheckDBErrCallback(callback, err, operationID)
			newConversation.ShowName = name
			newConversation.FaceURL = faceUrl
		case constant.GroupChatType:
			newConversation.GroupID = sourceID
			g, err := c.db.GetGroupInfoByGroupID(sourceID)
			common.CheckDBErrCallback(callback, err, operationID)
			newConversation.ShowName = g.GroupName
			newConversation.FaceURL = g.FaceURL
		}
		err := c.db.InsertConversation(&newConversation)
		common.CheckDBErrCallback(callback, err, operationID)
		return &newConversation
	}
}
func (c *Conversation) getMultipleConversation(callback open_im_sdk_callback.Base, conversationIDList []string, operationID string) sdk.GetMultipleConversationCallback {
	conversationList, err := c.db.GetMultipleConversation(conversationIDList)
	common.CheckDBErrCallback(callback, err, operationID)
	return conversationList
}

func (c *Conversation) deleteConversation(callback open_im_sdk_callback.Base, conversationID, operationID string) {
	lc, err := c.db.GetConversation(conversationID)
	common.CheckDBErrCallback(callback, err, operationID)
	var sourceID string
	switch lc.ConversationType {
	case constant.SingleChatType:
		sourceID = lc.UserID
	case constant.GroupChatType:
		sourceID = lc.GroupID
	}
	//Mark messages related to this conversation for deletion
	err = c.db.UpdateMessageStatusBySourceID(sourceID, constant.MsgStatusHasDeleted, lc.ConversationType)
	common.CheckDBErrCallback(callback, err, operationID)
	//Reset the session information, empty session
	err = c.db.ResetConversation(conversationID)
	common.CheckDBErrCallback(callback, err, operationID)
}
func (c *Conversation) setConversationDraft(callback open_im_sdk_callback.Base, conversationID, draftText, operationID string) {
	if draftText != "" {
		err := c.db.SetConversationDraft(conversationID, draftText)
		common.CheckDBErrCallback(callback, err, operationID)
	} else {
		err := c.db.RemoveConversationDraft(conversationID, draftText)
		common.CheckDBErrCallback(callback, err, operationID)
	}
}

func (c *Conversation) pinConversation(callback open_im_sdk_callback.Base, conversationID string, isPinned bool, operationID string) {
	lc := db.LocalConversation{ConversationID: conversationID, IsPinned: isPinned}
	if isPinned {
		err := c.db.UpdateConversation(&lc)
		common.CheckDBErrCallback(callback, err, operationID)
	} else {
		err := c.db.UnPinConversation(conversationID, constant.NotPinned)
		common.CheckDBErrCallback(callback, err, operationID)
	}
}

func (c *Conversation) getHistoryMessageList(callback open_im_sdk_callback.Base, req sdk.GetHistoryMessageListParams, operationID string) sdk.GetHistoryMessageListCallback {
	var sourceID string
	var conversationID string
	var startTime int64
	var sessionType int
	if req.UserID == "" {
		sourceID = req.GroupID
		conversationID = c.GetConversationIDBySessionType(sourceID, constant.GroupChatType)
		sessionType = constant.GroupChatType
	} else {
		sourceID = req.UserID
		conversationID = c.GetConversationIDBySessionType(sourceID, constant.SingleChatType)
		sessionType = constant.SingleChatType
	}
	if req.StartClientMsgID == "" {
		lc, err := c.db.GetConversation(conversationID)
		common.CheckDBErrCallback(callback, err, operationID)
		startTime = lc.LatestMsgSendTime + TimeOffset

	} else {
		m, err := c.db.GetMessage(req.StartClientMsgID)
		common.CheckDBErrCallback(callback, err, operationID)
		startTime = m.SendTime
	}
	log.Info(operationID, "sourceID:", sourceID, "startTime:", startTime, "count:", req.Count)
	list, err := c.db.GetMessageList(sourceID, sessionType, req.Count, startTime)
	common.CheckDBErrCallback(callback, err, operationID)
	return list
}
func (c *Conversation) revokeOneMessage(callback open_im_sdk_callback.Base, req sdk.RevokeMessageParams, operationID string) {
	var recvID, groupID string
	var localMessage db.LocalChatLog
	var lc db.LocalConversation
	var conversationID string
	message, err := c.db.GetMessage(req.ClientMsgID)
	common.CheckDBErrCallback(callback, err, operationID)
	if message.Status != constant.MsgStatusSendSuccess {
		common.CheckAnyErrCallback(callback, 201, errors.New("only send success message can be revoked"), operationID)
	}
	if message.SendID != c.loginUserID {
		common.CheckAnyErrCallback(callback, 201, errors.New("only you send message can be revoked"), operationID)
	}
	//Send message internally
	switch req.SessionType {
	case constant.SingleChatType:
		recvID = req.RecvID
		conversationID = c.GetConversationIDBySessionType(groupID, constant.SingleChatType)
	case constant.GroupChatType:
		groupID = req.GroupID
		conversationID = c.GetConversationIDBySessionType(groupID, constant.GroupChatType)
	default:
		common.CheckAnyErrCallback(callback, 201, errors.New("SessionType err"), operationID)
	}
	req.Content = message.ClientMsgID
	req.ClientMsgID = utils.GetMsgID(message.SendID)
	req.ContentType = constant.Revoke
	options := make(map[string]bool, 2)
	resp, _ := c.internalSendMessage(callback, (*sdk_struct.MsgStruct)(&req), recvID, groupID, operationID, &server_api_params.OfflinePushInfo{}, false, options)
	req.ServerMsgID = resp.ServerMsgID
	req.SendTime = resp.SendTime
	req.Status = constant.MsgStatusSendSuccess
	msgStructToLocalChatLog(&localMessage, (*sdk_struct.MsgStruct)(&req))
	err = c.db.InsertMessage(&localMessage)
	if err != nil {
		log.Error(operationID, "inset into chat log err", localMessage, req)
	}
	err = c.db.UpdateColumnsMessage(req.Content, map[string]interface{}{"status": constant.MsgStatusRevoked})
	if err != nil {
		log.Error(operationID, "update revoke message err", localMessage, req)
	}
	lc.LatestMsg = utils.StructToJsonString(req)
	lc.LatestMsgSendTime = req.SendTime
	lc.ConversationID = conversationID
	_ = common.TriggerCmdUpdateConversation(common.UpdateConNode{ConID: lc.ConversationID, Action: constant.AddConOrUpLatMsg, Args: lc}, c.ch)
}
func (c *Conversation) typingStatusUpdate(callback open_im_sdk_callback.Base, recvID, msgTip, operationID string) {
	s := sdk_struct.MsgStruct{}
	c.initBasicInfo(&s, constant.UserMsgType, constant.Typing, operationID)
	s.Content = msgTip
	options := make(map[string]bool, 2)
	c.internalSendMessage(callback, &s, recvID, "", operationID, &server_api_params.OfflinePushInfo{}, true, options)

}

func (c *Conversation) markC2CMessageAsRead(callback open_im_sdk_callback.Base, msgIDList sdk.MarkC2CMessageAsReadParams, sourceMsgIDList, userID, operationID string) {
	var localMessage db.LocalChatLog
	conversationID := c.GetConversationIDBySessionType(userID, constant.SingleChatType)
	s := sdk_struct.MsgStruct{}
	c.initBasicInfo(&s, constant.UserMsgType, constant.HasReadReceipt, operationID)
	s.Content = sourceMsgIDList
	options := make(map[string]bool, 2)
	resp, _ := c.internalSendMessage(callback, &s, userID, "", operationID, &server_api_params.OfflinePushInfo{}, false, options)
	s.ServerMsgID = resp.ServerMsgID
	s.SendTime = resp.SendTime
	s.Status = constant.MsgStatusSendSuccess
	msgStructToLocalChatLog(&localMessage, &s)
	err := c.db.InsertMessage(&localMessage)
	if err != nil {
		log.Error(operationID, "inset into chat log err", localMessage, s)
	}
	err2 := c.db.UpdateMessageHasRead(userID, msgIDList)
	if err2 != nil {
		log.Error(operationID, "update message has read error", msgIDList, userID)
	}
	_ = common.TriggerCmdUpdateConversation(common.UpdateConNode{ConID: conversationID, Action: constant.UpdateLatestMessageChange}, c.ch)
	_ = common.TriggerCmdUpdateConversation(common.UpdateConNode{ConID: conversationID, Action: constant.ConChange, Args: []string{conversationID}}, c.ch)
}
func (c *Conversation) insertMessageToLocalStorage(callback open_im_sdk_callback.Base, s *db.LocalChatLog, operationID string) string {
	err := c.db.InsertMessage(s)
	common.CheckDBErrCallback(callback, err, operationID)
	return s.ClientMsgID
}

func (c *Conversation) clearGroupHistoryMessage(callback open_im_sdk_callback.Base, groupID string, operationID string) {
	conversationID := c.GetConversationIDBySessionType(groupID, constant.GroupChatType)
	err := c.db.UpdateMessageStatusBySourceID(groupID, constant.MsgStatusHasDeleted, constant.GroupChatType)
	common.CheckDBErrCallback(callback, err, operationID)
	err = c.db.ClearConversation(conversationID)
	common.CheckDBErrCallback(callback, err, operationID)
	_ = common.TriggerCmdUpdateConversation(common.UpdateConNode{ConID: conversationID, Action: constant.ConChange, Args: []string{conversationID}}, c.ch)

}

func (c *Conversation) clearC2CHistoryMessage(callback open_im_sdk_callback.Base, userID string, operationID string) {
	conversationID := c.GetConversationIDBySessionType(userID, constant.SingleChatType)
	err := c.db.UpdateMessageStatusBySourceID(userID, constant.MsgStatusHasDeleted, constant.SingleChatType)
	common.CheckDBErrCallback(callback, err, operationID)
	err = c.db.ClearConversation(conversationID)
	common.CheckDBErrCallback(callback, err, operationID)
	_ = common.TriggerCmdUpdateConversation(common.UpdateConNode{ConID: conversationID, Action: constant.ConChange, Args: []string{conversationID}}, c.ch)
}

func (c *Conversation) deleteMessageFromLocalStorage(callback open_im_sdk_callback.Base, s *sdk_struct.MsgStruct, operationID string) {
	var conversation db.LocalConversation
	var latestMsg sdk_struct.MsgStruct
	var conversationID string
	var sourceID string
	chatLog := db.LocalChatLog{ClientMsgID: s.ClientMsgID, Status: constant.MsgStatusHasDeleted}
	err := c.db.UpdateMessage(&chatLog)
	common.CheckDBErrCallback(callback, err, operationID)

	callback.OnSuccess("")

	if s.SessionType == constant.GroupChatType {
		conversationID = c.GetConversationIDBySessionType(s.GroupID, constant.GroupChatType)
		sourceID = s.GroupID

	} else if s.SessionType == constant.SingleChatType {
		if s.SendID != c.loginUserID {
			conversationID = c.GetConversationIDBySessionType(s.SendID, constant.SingleChatType)
			sourceID = s.SendID
		} else {
			conversationID = c.GetConversationIDBySessionType(s.RecvID, constant.SingleChatType)
			sourceID = s.RecvID
		}
	}
	LocalConversation, err := c.db.GetConversation(conversationID)
	common.CheckDBErrCallback(callback, err, operationID)
	common.JsonUnmarshalCallback(LocalConversation.LatestMsg, &latestMsg, callback, operationID)

	if s.ClientMsgID == latestMsg.ClientMsgID { //If the deleted message is the latest message of the conversation, update the latest message of the conversation
		list, err := c.db.GetMessageList(sourceID, int(s.SessionType), 1, s.SendTime+TimeOffset)
		common.CheckDBErrCallback(callback, err, operationID)

		conversation.ConversationID = conversationID
		if list == nil {
			conversation.LatestMsg = ""
			conversation.LatestMsgSendTime = s.SendTime
		} else {
			conversation.LatestMsg = utils.StructToJsonString(list[0])
			conversation.LatestMsgSendTime = list[0].SendTime
		}
		_ = common.TriggerCmdUpdateConversation(common.UpdateConNode{ConID: conversation.ConversationID, Action: constant.AddConOrUpLatMsg, Args: conversation}, c.ch)
		_ = common.TriggerCmdUpdateConversation(common.UpdateConNode{ConID: conversationID, Action: constant.ConChange, Args: []string{conversationID}}, c.ch)
	}
}