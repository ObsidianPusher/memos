import { Tooltip } from "@mui/joy";
import { BookmarkIcon, EyeOffIcon, MessageCircleMoreIcon } from "lucide-react";
import { observer } from "mobx-react-lite";
import { memo, useCallback, useEffect, useState } from "react";
import { Link, useLocation } from "react-router-dom";
import useCurrentUser from "@/hooks/useCurrentUser";
import useNavigateTo from "@/hooks/useNavigateTo";
import { memoStore, userStore, workspaceStore } from "@/store/v2";
import { State } from "@/types/proto/api/v1/common";
import { Emotion, Memo, MemoRelation_Type, Visibility } from "@/types/proto/api/v1/memo_service";
import { cn } from "@/utils";
import { EMOTION_META } from "@/utils/emotion";
import { useTranslate } from "@/utils/i18n";
import { convertVisibilityToString } from "@/utils/memo";
import { isSuperUser } from "@/utils/user";
import MemoActionMenu from "./MemoActionMenu";
import MemoContent from "./MemoContent";
import MemoEditor from "./MemoEditorLazy";
import MemoRelationListView from "./MemoRelationListView";
import MemoResourceListView from "./MemoResourceListView";
import showPreviewImageDialog from "./PreviewImageDialog";
import UserAvatar from "./UserAvatar";
import VisibilityIcon from "./VisibilityIcon";

interface Props {
  memo: Memo;
  compact?: boolean;
  showCreator?: boolean;
  showVisibility?: boolean;
  showPinned?: boolean;
  showNsfwContent?: boolean;
  className?: string;
  parentPage?: string;
}

const MemoView: React.FC<Props> = observer((props: Props) => {
  const { memo, className } = props;
  const t = useTranslate();
  const location = useLocation();
  const navigateTo = useNavigateTo();
  const user = useCurrentUser();
  const [showEditor, setShowEditor] = useState<boolean>(false);
  const [creator, setCreator] = useState(userStore.getUserByName(memo.creator));
  const [showNSFWContent, setShowNSFWContent] = useState(props.showNsfwContent);
  const workspaceMemoRelatedSetting = workspaceStore.state.memoRelatedSetting;
  const referencedMemos = memo.relations.filter((relation) => relation.type === MemoRelation_Type.REFERENCE);
  const commentAmount = memo.relations.filter(
    (relation) => relation.type === MemoRelation_Type.COMMENT && relation.relatedMemo?.name === memo.name,
  ).length;
  const relativeTimeFormat = Date.now() - memo.displayTime!.getTime() > 1000 * 60 * 60 * 24 ? "datetime" : "auto";
  const isArchived = memo.state === State.ARCHIVED;
  const readonly = memo.creator !== user?.name && !isSuperUser(user);
  const isInMemoDetailPage = location.pathname.startsWith(`/${memo.name}`);
  const parentPage = props.parentPage || location.pathname;
  const nsfw =
    workspaceMemoRelatedSetting.enableBlurNsfwContent &&
    memo.tags?.some((tag) => workspaceMemoRelatedSetting.nsfwTags.some((nsfwTag) => tag === nsfwTag || tag.startsWith(`${nsfwTag}/`)));

  useEffect(() => {
    setCreator(userStore.getUserByName(memo.creator));
  }, [memo.creator]);

  useEffect(() => {
    if (!props.showCreator) {
      return;
    }

    if (creator) {
      return;
    }

    let cancelled = false;
    void userStore.getOrFetchUserByName(memo.creator).then((user) => {
      if (!cancelled) {
        setCreator(user);
      }
    });

    return () => {
      cancelled = true;
    };
  }, [creator, memo.creator, props.showCreator]);

  const handleGotoMemoDetailPage = useCallback(() => {
    navigateTo(`/${memo.name}`, {
      state: {
        from: parentPage,
      },
    });
  }, [memo.name, parentPage]);

  const handleMemoContentClick = useCallback(async (e: React.MouseEvent) => {
    const targetEl = e.target as HTMLElement;

    if (targetEl.tagName === "IMG") {
      const imgUrl = targetEl.getAttribute("src");
      if (imgUrl) {
        showPreviewImageDialog([imgUrl], 0);
      }
    }
  }, []);

  const handleMemoContentDoubleClick = useCallback(async (e: React.MouseEvent) => {
    if (readonly) {
      return;
    }

    if (workspaceMemoRelatedSetting.enableDoubleClickEdit) {
      e.preventDefault();
      setShowEditor(true);
    }
  }, []);

  const onEditorConfirm = () => {
    setShowEditor(false);
    userStore.setStatsStateId();
  };

  const onPinIconClick = async () => {
    if (memo.pinned) {
      await memoStore.updateMemo(
        {
          name: memo.name,
          pinned: false,
        },
        ["pinned"],
      );
    }
  };

  const displayTime = isArchived ? (
    memo.displayTime?.toLocaleString()
  ) : (
    <relative-time datetime={memo.displayTime?.toISOString()} format={relativeTimeFormat}></relative-time>
  );
  const emotionMeta = EMOTION_META[memo.emotion];
  const showEmotion = memo.emotion !== Emotion.EMOTION_UNSPECIFIED && memo.emotion !== Emotion.UNRECOGNIZED;

  return showEditor ? (
    <MemoEditor
      autoFocus
      className="mb-2"
      cacheKey={`inline-memo-editor-${memo.name}`}
      memoName={memo.name}
      onConfirm={onEditorConfirm}
      onCancel={() => setShowEditor(false)}
    />
  ) : (
    <div
      className={cn(
        "group relative flex flex-col justify-start items-start w-full px-4 py-3 mb-2 gap-2 bg-white dark:bg-zinc-800 rounded-lg border border-white dark:border-zinc-800 hover:border-gray-200 dark:hover:border-zinc-700",
        className,
      )}
    >
      <div className="w-full flex flex-row justify-between items-center gap-2">
        <div className="w-auto max-w-[calc(100%-8rem)] grow flex flex-row justify-start items-center">
          {props.showCreator && creator ? (
            <div className="w-full flex flex-row justify-start items-center">
              <Link className="w-auto hover:opacity-80" to={`/u/${encodeURIComponent(creator.username)}`} viewTransition>
                <UserAvatar className="mr-2 shrink-0" avatarUrl={creator.avatarUrl} />
              </Link>
              <div className="w-full flex flex-col justify-center items-start">
                <Link
                  className="w-full block leading-tight hover:opacity-80 truncate text-gray-600 dark:text-gray-400"
                  to={`/u/${encodeURIComponent(creator.username)}`}
                  viewTransition
                >
                  {creator.nickname || creator.username}
                </Link>
                <div className="w-full -mt-0.5 flex flex-wrap items-center gap-2 text-xs leading-tight text-gray-400 dark:text-gray-500 select-none">
                  <span className="cursor-pointer" onClick={handleGotoMemoDetailPage}>
                    {displayTime}
                  </span>
                  {showEmotion && (
                    <span
                      className={cn("inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[0.7rem] font-medium", emotionMeta.color)}
                      title={t(emotionMeta.labelKey as any)}
                    >
                      <span>{emotionMeta.emoji}</span>
                      <span>{t(emotionMeta.labelKey as any)}</span>
                    </span>
                  )}
                </div>
              </div>
            </div>
          ) : (
            <div
              className="w-full flex flex-wrap items-center gap-2 text-sm leading-tight text-gray-400 dark:text-gray-500 select-none cursor-pointer"
              onClick={handleGotoMemoDetailPage}
            >
              <span>{displayTime}</span>
              {showEmotion && (
                <span
                  className={cn("inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[0.7rem] font-medium", emotionMeta.color)}
                >
                  <span>{emotionMeta.emoji}</span>
                  <span>{t(emotionMeta.labelKey as any)}</span>
                </span>
              )}
            </div>
          )}
        </div>
        <div className="flex flex-row justify-end items-center select-none shrink-0 gap-2">
          <div className="w-auto invisible group-hover:visible flex flex-row justify-between items-center gap-2">
            {props.showVisibility && memo.visibility !== Visibility.PRIVATE && (
              <Tooltip title={t(`memo.visibility.${convertVisibilityToString(memo.visibility).toLowerCase()}` as any)} placement="top">
                <span className="flex justify-center items-center hover:opacity-70">
                  <VisibilityIcon visibility={memo.visibility} />
                </span>
              </Tooltip>
            )}
          </div>
          {!isInMemoDetailPage && (workspaceMemoRelatedSetting.enableComment || commentAmount > 0) && (
            <Link
              className={cn(
                "flex flex-row justify-start items-center hover:opacity-70",
                commentAmount === 0 && "invisible group-hover:visible",
              )}
              to={`/${memo.name}#comments`}
              viewTransition
              state={{
                from: parentPage,
              }}
            >
              <MessageCircleMoreIcon className="w-4 h-4 mx-auto text-gray-500 dark:text-gray-400" />
              {commentAmount > 0 && <span className="text-xs text-gray-500 dark:text-gray-400">{commentAmount}</span>}
            </Link>
          )}
          {props.showPinned && memo.pinned && (
            <Tooltip title={t("common.unpin")} placement="top">
              <span className="cursor-pointer">
                <BookmarkIcon className="w-4 h-auto text-amber-500" onClick={onPinIconClick} />
              </span>
            </Tooltip>
          )}
          {nsfw && showNSFWContent && (
            <span className="cursor-pointer">
              <EyeOffIcon className="w-4 h-auto text-amber-500" onClick={() => setShowNSFWContent(false)} />
            </span>
          )}
          <MemoActionMenu className="-ml-1" memo={memo} readonly={readonly} onEdit={() => setShowEditor(true)} />
        </div>
      </div>
      <div
        className={cn(
          "w-full flex flex-col justify-start items-start gap-2",
          nsfw && !showNSFWContent && "blur-lg transition-all duration-200",
        )}
      >
        <MemoContent
          memoName={memo.name}
          nodes={memo.nodes}
          readonly={readonly}
          onClick={handleMemoContentClick}
          onDoubleClick={handleMemoContentDoubleClick}
          compact={memo.pinned ? false : props.compact} // Always show full content when pinned.
          parentPage={parentPage}
        />
        <MemoResourceListView resources={memo.resources} />
        <MemoRelationListView memo={memo} relations={referencedMemos} parentPage={parentPage} />
      </div>
      {nsfw && !showNSFWContent && (
        <>
          <div className="absolute inset-0 bg-transparent" />
          <button
            className="absolute top-1/2 left-1/2 transform -translate-x-1/2 -translate-y-1/2 py-2 px-4 text-sm text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300 border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-zinc-800"
            onClick={() => setShowNSFWContent(true)}
          >
            {t("memo.click-to-show-nsfw-content")}
          </button>
        </>
      )}
    </div>
  );
});

export default memo(MemoView);
