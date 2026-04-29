import { Button, Input, Textarea } from "@mui/joy";
import { useState } from "react";
import { toast } from "react-hot-toast";
import * as api from "@/helpers/api";
import { useTranslate } from "@/utils/i18n";
import { generateDialog } from "./Dialog";
import Icon from "./Icon";

interface Props extends DialogProps {
  shortcut?: Shortcut;
  onSuccess?: () => void;
}

const CreateShortcutDialog: React.FC<Props> = (props: Props) => {
  const { destroy, shortcut, onSuccess } = props;
  const t = useTranslate();
  const [title, setTitle] = useState(shortcut?.title ?? "");
  const [payload, setPayload] = useState(shortcut?.payload ?? "");
  const [requesting, setRequesting] = useState(false);
  const isCreating = !shortcut;

  const handleSaveBtnClick = async () => {
    const normalizedTitle = title.trim();
    const normalizedPayload = payload.trim();
    if (!normalizedTitle || !normalizedPayload) {
      toast.error("Title and filter cannot be empty");
      return;
    }

    try {
      setRequesting(true);
      if (shortcut) {
        await api.patchShortcut({
          id: shortcut.id,
          title: normalizedTitle,
          payload: normalizedPayload,
        });
        toast.success("Updated shortcut successfully");
      } else {
        await api.createShortcut({
          title: normalizedTitle,
          payload: normalizedPayload,
        });
        toast.success("Created shortcut successfully");
      }
      onSuccess?.();
      destroy();
    } catch (error: any) {
      console.error(error);
      toast.error(error.response?.data?.message ?? (isCreating ? "Failed to create shortcut" : "Failed to update shortcut"));
    } finally {
      setRequesting(false);
    }
  };

  return (
    <>
      <div className="dialog-header-container">
        <p className="title-text">{isCreating ? t("common.create") : t("common.edit")} Shortcut</p>
        <button className="btn close-btn" onClick={() => destroy()}>
          <Icon.X />
        </button>
      </div>
      <div className="dialog-content-container !w-96">
        <div className="w-full flex flex-col gap-3">
          <Input autoFocus placeholder="Title" value={title} onChange={(event) => setTitle(event.target.value)} />
          <Textarea
            minRows={3}
            placeholder='Filter, e.g. tag in ["tag1", "tag2"]'
            value={payload}
            onChange={(event) => setPayload(event.target.value)}
          />
          <a
            className="text-sm text-blue-600 dark:text-blue-400 hover:underline"
            href="https://usememos.com/docs/usage/shortcuts"
            target="_blank"
            rel="noopener noreferrer"
          >
            Docs - Shortcuts
          </a>
          <div className="w-full flex justify-end gap-2">
            <Button variant="plain" color="neutral" disabled={requesting} onClick={() => destroy()}>
              {t("common.cancel")}
            </Button>
            <Button disabled={requesting} onClick={handleSaveBtnClick}>
              {t("common.save")}
            </Button>
          </div>
        </div>
      </div>
    </>
  );
};

function showCreateShortcutDialog(shortcut?: Shortcut, onSuccess?: () => void) {
  generateDialog(
    {
      className: "create-shortcut-dialog",
      dialogName: "create-shortcut-dialog",
    },
    CreateShortcutDialog,
    {
      shortcut,
      onSuccess,
    }
  );
}

export default showCreateShortcutDialog;
