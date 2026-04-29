import { LINK_REG, PLAIN_LINK_REG, TAG_REG } from "@/labs/marked/parser";

type ShortcutCondition =
  | { type: "TAG_IN"; values: string[] }
  | { type: "CONTENT_CONTAINS"; value: string; normalizedValue: string }
  | { type: "VISIBILITY_IN"; values: string[] }
  | { type: "HAS_LINK" }
  | { type: "HAS_TASK_LIST" }
  | { type: "HAS_CODE" }
  | { type: "PINNED" }
  | { type: "CREATED_TS_COMPARE"; operator: CreatedTsOperator; value: number };

type CreatedTsOperator = ">=" | ">" | "<=" | "<";

const TODO_REG = /- \[[ x]\] /;
const CODE_BLOCK_REG = /```[\s\S]*?```/;
const INLINE_CODE_REG = /`[^`]+`/;

const normalizeCreatedTsValue = (value: number): number => {
  return value < 1_000_000_000_000 ? value * 1000 : value;
};

const splitConditions = (filter: string): string[] => {
  const conditions: string[] = [];
  let current = "";
  let inString = false;
  let bracketDepth = 0;

  for (let i = 0; i < filter.length; i++) {
    const char = filter[i];
    const next = filter[i + 1];
    if (char === '"' && (i === 0 || filter[i - 1] !== "\\")) {
      inString = !inString;
    }
    if (!inString) {
      if (char === "[") {
        bracketDepth++;
      } else if (char === "]") {
        bracketDepth--;
      } else if (char === "&" && next === "&" && bracketDepth === 0) {
        conditions.push(current.trim());
        current = "";
        i++;
        continue;
      }
    }
    current += char;
  }

  if (current.trim()) {
    conditions.push(current.trim());
  }

  return conditions;
};

// Parses a quoted JSON string literal such as `"tag1"` and returns its string value.
const parseStringLiteral = (value: string): string | undefined => {
  try {
    const parsed = JSON.parse(value);
    return typeof parsed === "string" ? parsed : undefined;
  } catch {
    return undefined;
  }
};

const parseStringArray = (value: string): string[] | undefined => {
  try {
    const parsed = JSON.parse(`[${value}]`);
    return Array.isArray(parsed) && parsed.every((item) => typeof item === "string") ? parsed : undefined;
  } catch {
    return undefined;
  }
};

const parseCondition = (condition: string): ShortcutCondition | undefined => {
  let matched = condition.match(/^tag\s+in\s+\[(.*)\]$/i);
  if (matched) {
    const values = parseStringArray(matched[1]);
    return values && values.length > 0 ? { type: "TAG_IN", values } : undefined;
  }

  matched = condition.match(/^content\.contains\((.*)\)$/i);
  if (matched) {
    const value = parseStringLiteral(matched[1].trim());
    return value ? { type: "CONTENT_CONTAINS", value, normalizedValue: value.toLowerCase() } : undefined;
  }

  matched = condition.match(/^visibility\s+in\s+\[(.*)\]$/i);
  if (matched) {
    const values = parseStringArray(matched[1]);
    return values && values.length > 0 ? { type: "VISIBILITY_IN", values: values.map((value) => value.toUpperCase()) } : undefined;
  }

  matched = condition.match(/^created_ts\s*(>=|>|<=|<)\s*(\d+(?:\.\d+)?)$/i);
  if (matched) {
    return { type: "CREATED_TS_COMPARE", operator: matched[1] as CreatedTsOperator, value: normalizeCreatedTsValue(Number(matched[2])) };
  }

  if (condition === "has_link") {
    return { type: "HAS_LINK" };
  }
  if (condition === "has_task_list") {
    return { type: "HAS_TASK_LIST" };
  }
  if (condition === "has_code") {
    return { type: "HAS_CODE" };
  }
  if (condition === "pinned") {
    return { type: "PINNED" };
  }

  return undefined;
};

export const parseShortcutExpressionFilter = (filter: string): ShortcutCondition[] | undefined => {
  const conditions = splitConditions(filter);
  if (conditions.length === 0) {
    return undefined;
  }

  const parsedConditions = conditions.map(parseCondition);
  if (parsedConditions.some((condition) => condition === undefined)) {
    return undefined;
  }
  return parsedConditions as ShortcutCondition[];
};

const getMemoTags = (memo: Memo): Set<string> => {
  const tagsSet = new Set<string>();
  for (const matched of Array.from(memo.content.match(new RegExp(TAG_REG, "gu")) ?? [])) {
    const tag = matched.replace(TAG_REG, "$1").trim();
    const items = tag.split("/");
    let temp = "";
    for (const item of items) {
      temp += item;
      tagsSet.add(temp);
      temp += "/";
    }
  }
  return tagsSet;
};

const evaluateCreatedTsCondition = (createdTs: number, operator: CreatedTsOperator, value: number): boolean => {
  if (operator === ">=") {
    return createdTs >= value;
  }
  if (operator === ">") {
    return createdTs > value;
  }
  if (operator === "<=") {
    return createdTs <= value;
  }
  return createdTs < value;
};

export const matchShortcutExpressionFilter = (memo: Memo, filter: string): boolean => {
  const conditions = parseShortcutExpressionFilter(filter);
  if (!conditions) {
    return false;
  }

  return conditions.every((condition) => {
    if (condition.type === "TAG_IN") {
      const tags = getMemoTags(memo);
      return condition.values.some((tag) => tags.has(tag));
    }
    if (condition.type === "CONTENT_CONTAINS") {
      return memo.content.toLowerCase().includes(condition.normalizedValue);
    }
    if (condition.type === "VISIBILITY_IN") {
      return condition.values.includes(memo.visibility);
    }
    if (condition.type === "HAS_LINK") {
      return memo.content.match(LINK_REG) !== null || memo.content.match(PLAIN_LINK_REG) !== null;
    }
    if (condition.type === "HAS_TASK_LIST") {
      return memo.content.match(TODO_REG) !== null;
    }
    if (condition.type === "HAS_CODE") {
      return memo.content.match(CODE_BLOCK_REG) !== null || memo.content.match(INLINE_CODE_REG) !== null;
    }
    if (condition.type === "PINNED") {
      return memo.pinned;
    }
    if (condition.type === "CREATED_TS_COMPARE") {
      return evaluateCreatedTsCondition(memo.createdTs, condition.operator, condition.value);
    }

    return false;
  });
};
