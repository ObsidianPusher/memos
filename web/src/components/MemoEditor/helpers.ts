import { isEqual } from "lodash-es";
import { Emotion, Memo, MemoRelation, Visibility } from "@/types/proto/api/v1/memo_service";
import { Resource } from "@/types/proto/api/v1/resource_service";

export interface MemoEditorState {
  memoVisibility: Visibility;
  emotion: Emotion;
  resourceList: Resource[];
  relationList: MemoRelation[];
  isUploadingResource: boolean;
  isRequesting: boolean;
  isComposing: boolean;
  isDraggingFile: boolean;
}

export const createInitialMemoEditorState = (): MemoEditorState => ({
  memoVisibility: Visibility.PRIVATE,
  emotion: Emotion.EMOTION_UNSPECIFIED,
  resourceList: [],
  relationList: [],
  isUploadingResource: false,
  isRequesting: false,
  isComposing: false,
  isDraggingFile: false,
});

export const buildMemoUpdate = ({
  prevMemo,
  content,
  memoVisibility,
  emotion,
  resourceList,
  relationList,
  createTime,
  updateTime,
}: {
  prevMemo: Memo;
  content: string;
  memoVisibility: Visibility;
  emotion: Emotion;
  resourceList: Resource[];
  relationList: MemoRelation[];
  createTime?: Date;
  updateTime?: Date;
}) => {
  const updateMask = new Set<string>();
  const memoPatch: Partial<Memo> = {
    name: prevMemo.name,
    content,
    emotion,
  };

  if (!isEqual(content, prevMemo.content)) {
    updateMask.add("content");
    memoPatch.content = content;
  }
  if (!isEqual(memoVisibility, prevMemo.visibility)) {
    updateMask.add("visibility");
    memoPatch.visibility = memoVisibility;
  }
  if (!isEqual(resourceList, prevMemo.resources)) {
    updateMask.add("resources");
    memoPatch.resources = resourceList;
  }
  if (!isEqual(relationList, prevMemo.relations)) {
    updateMask.add("relations");
    memoPatch.relations = relationList;
  }
  if (!isEqual(emotion, prevMemo.emotion)) {
    updateMask.add("emotion");
    memoPatch.emotion = emotion;
  }
  if (["content", "resources", "relations", "emotion"].some((key) => updateMask.has(key))) {
    updateMask.add("update_time");
  }
  if (createTime && !isEqual(createTime, prevMemo.createTime)) {
    updateMask.add("create_time");
    memoPatch.createTime = createTime;
  }
  if (updateTime && !isEqual(updateTime, prevMemo.updateTime)) {
    updateMask.add("update_time");
    memoPatch.updateTime = updateTime;
  }

  return {
    memoPatch,
    updateMask: Array.from(updateMask),
  };
};
