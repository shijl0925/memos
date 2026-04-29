type ShortcutId = number;

interface Shortcut {
  id: ShortcutId;
  rowStatus: RowStatus;
  creatorId: number;
  createdTs: TimeStamp;
  updatedTs: TimeStamp;
  title: string;
  payload: string;
}

interface ShortcutCreate {
  title: string;
  payload: string;
}

interface ShortcutPatch {
  id: ShortcutId;
  title?: string;
  payload?: string;
  rowStatus?: RowStatus;
}

interface ShortcutFilter {
  id: ShortcutId;
  title: string;
  payload: string;
}
