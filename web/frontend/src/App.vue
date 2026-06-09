<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";

const VISIT_KEY = "notion-like:recent-visits";
const FAVORITE_KEY = "notion-like:favorites";
const AI_THREAD_KEY = "notion-like:ai-threads";
const PLAN_TASK_KEY = "notion-like:plan-tasks";
const TEMPLATE_PREF_KEY = "notion-like:template-prefs";
const AI_THREAD_TTL_MS = 7 * 24 * 60 * 60 * 1000;

const notes = ref([]);
const allNotes = ref([]);
const archivedNotes = ref([]);
const noteDetailCache = ref(new Map());

const activeView = ref("home");
const noteMode = ref("preview");
const selectedId = ref(null);
const selectedNote = ref(null);
const selectedFolderID = ref(null);

const title = ref("");
const markdown = ref("");
const selectedTags = ref([]);
const parentID = ref("");
const previewHTML = ref("");

const searchTimer = ref(null);
const previewTimer = ref(null);
const autosaveTimer = ref(null);
const searchModalTimer = ref(null);
const sidebarPeekCloseTimer = ref(null);
const historyTimer = ref(null);

const hydrating = ref(false);
const isSaving = ref(false);
const saveState = ref("idle");
const lastSavedSignature = ref("");

const sidebarPinned = ref(true);
const sidebarPeek = ref(false);
const privateExpanded = ref(true);
const folderOpen = ref({});

const searchModalOpen = ref(false);
const searchText = ref("");
const searchResults = ref([]);
const searchLoading = ref(false);
const searchInputRef = ref(null);
const searchSort = ref("relevance");
const searchOnlyTitle = ref(false);
const searchInPage = ref(false);
const searchDate = ref("all");
const searchCursor = ref(-1);
const searchTags = ref([]);
const searchTagMenuOpen = ref(false);
const searchTagMenuRef = ref(null);

const trashModalOpen = ref(false);
const trashQuery = ref("");

const recentVisits = ref([]);
const recentStripRef = ref(null);
const recentHover = ref(false);
const canRecentLeft = ref(false);
const canRecentRight = ref(false);
const favoriteIDs = ref([]);
const favoriteStripRef = ref(null);
const favoriteHover = ref(false);
const canFavoriteLeft = ref(false);
const canFavoriteRight = ref(false);

const draggingNoteID = ref(null);
const dragOverNoteID = ref(null);

const contextMenuOpen = ref(false);
const contextMenuX = ref(0);
const contextMenuY = ref(0);
const contextNoteID = ref(null);
const tagMenuOpen = ref(false);
const tagQuery = ref("");
const tagMenuRef = ref(null);
const allTags = ref([]);

const aiOpen = ref(false);
const aiInput = ref("");
const aiMessages = ref([]);
const aiLoading = ref(false);
const aiMessagesRef = ref(null);
const assistantInputRef = ref(null);
const assistantAttachmentFileInputRef = ref(null);
const assistantThreadKey = ref("assistant:main");
const assistantScope = ref("library");
const aiHistoryOpen = ref(false);
const assistantAttachmentMenuOpen = ref(false);
const assistantNotePickerOpen = ref(false);
const assistantNoteSearch = ref("");
const assistantAttachmentReading = ref(false);
const assistantAttachments = ref([]);
const aiOptimizing = ref(false);
const aiOptimizeMessage = ref("");
const aiThreads = ref(new Map());

const editorTextareaRef = ref(null);
const titleInputRef = ref(null);
const speechRecognitionRef = ref(null);
const voiceState = ref("idle");
const voiceTranscript = ref("");
const voiceInterim = ref("");
const voiceError = ref("");
const voicePausePending = ref(false);
const voiceFinishPending = ref(false);
const undoStack = ref([]);
const redoStack = ref([]);
const historyApplying = ref(false);
const lastHistorySignature = ref("");

const reviewReport = ref(null);
const taskItems = ref([]);
const templates = ref([]);
const workspaceDashboard = ref(null);
const workspaceDashboardLoading = ref(false);
const knowledgeCards = ref([]);
const dueKnowledgeCards = ref([]);
const cardSearch = ref("");
const cardFilter = ref("active");
const cardFormOpen = ref(false);
const editingCardID = ref(0);
const cardForm = ref({ front: "", back: "" });
const cardSaving = ref(false);
const cardReviewIndex = ref(0);
const cardAnswerVisible = ref(false);
const cardReviewing = ref(false);
const reviewSessionCards = ref([]);
const reviewSessionDone = ref(0);
const workspaceGraph = ref({ nodes: [], edges: [] });
const graphLoading = ref(false);
const graphRelationFilter = ref("important");
const graphTagFilter = ref("");
const graphQuery = ref("");
const graphFocusID = ref(0);
const researchTopic = ref("");
const researchLoading = ref(false);
const researchResult = ref(null);
const researchCardCreating = ref("");
const researchHistory = ref([]);
const researchHistoryOpen = ref(false);
const noteInsights = ref(null);
const intelligenceLoading = ref(false);
const intelligenceMessage = ref("");
const intelligenceRequestToken = ref(0);
const reviewQuestionFormOpen = ref(false);
const reviewQuestionForm = ref({ id: 0, question: "", answer: "" });
const reviewQuestionSaving = ref(false);
const reviewQuestionGenerating = ref(false);
const reviewQuestionDeletingID = ref(0);
const reviewQuestionError = ref("");
const homeIntelligenceLoading = ref(false);
const importModalOpen = ref(false);
const importTitle = ref("");
const importText = ref("");
const importTags = ref([]);
const importTagMenuOpen = ref(false);
const importTagQuery = ref("");
const importFileName = ref("");
const importParentID = ref("");
const importReading = ref(false);
const importFileInputRef = ref(null);
const planTasks = ref([]);
const planTaskTitle = ref("");
const planTaskDate = ref("");
const planTaskStartTime = ref("09:00");
const planTaskEndTime = ref("10:00");
const planTaskPriority = ref("medium");
const planTaskFilter = ref("open");
const calendarSelectedDate = ref(new Date().toISOString().slice(0, 10));
const calendarSelectedTaskID = ref(null);
const calendarCommandOpen = ref(false);
const calendarCommandQuery = ref("");
const calendarCommandInputRef = ref(null);
const calendarCommandCursor = ref(0);
const calendarPlanSearchOpen = ref(false);
const calendarPlanSearchQuery = ref("");
const calendarPlanSearchInputRef = ref(null);
const calendarPlanSearchCursor = ref(0);
const calendarContextMenuOpen = ref(false);
const calendarContextMenuX = ref(0);
const calendarContextMenuY = ref(0);
const calendarContextDate = ref("");
const calendarContextHour = ref(9);
const templatePrefs = ref({ custom: [], deleted: [] });
const templateModalOpen = ref(false);
const editingTemplateKey = ref("");
const templateForm = ref({ name: "", tags: [], markdown: "" });
const templateTagMenuOpen = ref(false);
const templateTagQuery = ref("");
const recommendQuery = ref("");
const recommendNotePickerOpen = ref(false);
const recommendNoteSearch = ref("");
const recommendSelectedIDs = ref([]);
const recommendLoading = ref(false);
const recommendResult = ref(null);
const recommendHistory = ref([]);
const recommendHistoryOpen = ref(false);
const weeklyReportTitle = ref(defaultWeeklyReportTitle());
const weeklyReportParentID = ref("");
const weeklyReportLoading = ref(false);
const weeklyReportResult = ref(null);
const weeklyNotePickerOpen = ref(false);
const weeklyNoteSearch = ref("");
const weeklySelectedIDs = ref([]);
const weeklyLocalFiles = ref([]);
const weeklyFileInputRef = ref(null);
const weeklyFileReading = ref(false);
const quickMemoTemplateKey = ref("");
const quickMemoParentID = ref("");
const quickMemoText = ref("");
const quickMemoInterim = ref("");
const quickMemoState = ref("idle");
const quickMemoError = ref("");
const quickMemoSaving = ref(false);
const quickMemoResult = ref(null);
const quickMemoRecognitionRef = ref(null);
const quickMemoFinishPending = ref(false);
const guideModalOpen = ref(false);
const currentGuideKey = ref("workspace");

const workspaceName = "本地笔记";
const mermaidStartRE = /^(flowchart|graph|sequenceDiagram|classDiagram|stateDiagram(?:-v2)?|erDiagram|journey|gantt|pie|gitGraph|mindmap|timeline|quadrantChart|xychart-beta|sankey-beta|block-beta|packet-beta|architecture-beta|requirementDiagram|C4Context|C4Container|C4Component|C4Dynamic)\b/;

let mermaidConfigured = false;
let mermaidRenderToken = 0;
let mermaidLoader = null;

const notesByID = computed(() => {
  const m = new Map();
  for (const n of allNotes.value || []) m.set(n.id, n);
  return m;
});

const activeNotes = computed(() => (allNotes.value || []).filter((n) => !n.is_archived));
const contentNotes = computed(() => (activeNotes.value || []).filter((n) => !isFolder(n)));

const pendingTasks = computed(() => (planTasks.value || []).filter((task) => !task.done));
const completedTasks = computed(() => (planTasks.value || []).filter((task) => task.done));
const todayPlanTasks = computed(() => {
  const today = new Date().toISOString().slice(0, 10);
  return (planTasks.value || []).filter((task) => task.due === today && !task.done);
});
const highPlanTasks = computed(() => (planTasks.value || []).filter((task) => task.priority === "high" && !task.done));
const recommendedNext = computed(() => reviewReport.value?.recommended_next || []);

const folderOptions = computed(() => {
  return (activeNotes.value || [])
    .filter((n) => isFolder(n))
    .sort((a, b) => noteFullPath(a).localeCompare(noteFullPath(b), "zh-CN"))
    .map((n) => ({ id: n.id, title: folderSelectLabel(n) }));
});

const workspaceTemplates = computed(() => {
  const deleted = new Set(templatePrefs.value.deleted || []);
  const custom = templatePrefs.value.custom || [];
  const customByKey = new Map(custom.map((tpl) => [tpl.key, tpl]));
  const merged = [];
  for (const tpl of templates.value || []) {
    if (deleted.has(tpl.key)) continue;
    merged.push(customByKey.get(tpl.key) || tpl);
  }
  for (const tpl of custom) {
    if (!(templates.value || []).some((base) => base.key === tpl.key) && !deleted.has(tpl.key)) {
      merged.push(tpl);
    }
  }
  return merged;
});

const filteredPlanTasks = computed(() => {
  const list = [...(planTasks.value || [])];
  if (planTaskFilter.value === "open") return list.filter((task) => !task.done);
  if (planTaskFilter.value === "done") return list.filter((task) => task.done);
  if (planTaskFilter.value === "today") {
    const today = new Date().toISOString().slice(0, 10);
    return list.filter((task) => task.due === today);
  }
  return list;
});

const templateTagSet = computed(() => new Set((templateForm.value.tags || []).map((tag) => String(tag).toLowerCase())));
const templateTagOptions = computed(() => {
  const q = String(templateTagQuery.value || "").trim().toLowerCase();
  const list = normalizeTags(allTags.value || []);
  if (!q) return list;
  return list.filter((tag) => String(tag).toLowerCase().includes(q));
});
const canCreateTemplateTag = computed(() => {
  const tag = String(templateTagQuery.value || "").trim();
  if (!tag) return false;
  return !templateTagSet.value.has(tag.toLowerCase());
});

const importTagSet = computed(() => new Set((importTags.value || []).map((tag) => String(tag).toLowerCase())));
const importTagOptions = computed(() => {
  const q = String(importTagQuery.value || "").trim().toLowerCase();
  const list = normalizeTags(allTags.value || []);
  if (!q) return list;
  return list.filter((tag) => String(tag).toLowerCase().includes(q));
});
const canCreateImportTag = computed(() => {
  const tag = String(importTagQuery.value || "").trim();
  if (!tag) return false;
  return !importTagSet.value.has(tag.toLowerCase());
});

const planProgress = computed(() => {
  const total = planTasks.value.length;
  if (!total) return 0;
  return Math.round((completedTasks.value.length / total) * 100);
});

const isMacLike = computed(() => {
  if (typeof navigator === "undefined") return true;
  return /mac|iphone|ipad|ipod/i.test(navigator.platform || navigator.userAgent || "");
});

const calendarShortcutLabel = computed(() => (isMacLike.value ? "command K" : "alt K"));
const calendarSelectedDateText = computed(() => formatCalendarDateLong(calendarSelectedDate.value));
const calendarMonthLabel = computed(() => formatCalendarMonth(calendarSelectedDate.value));
const calendarMonthDays = computed(() => buildCalendarMonth(calendarSelectedDate.value));
const calendarWeekDays = computed(() => buildCalendarWeek(calendarSelectedDate.value));
const calendarHours = computed(() => Array.from({ length: 12 }, (_, i) => i + 7));
const calendarSelectedDateTasks = computed(() => tasksForDate(calendarSelectedDate.value));
const selectedCalendarTask = computed(() => {
  const id = Number(calendarSelectedTaskID.value || 0);
  if (!id) return null;
  return (planTasks.value || []).find((task) => Number(task.id) === id) || null;
});
const calendarPlanSearchResults = computed(() => {
  const q = String(calendarPlanSearchQuery.value || "").trim().toLowerCase();
  const list = [...(planTasks.value || [])].sort((a, b) => {
    const aKey = `${a.due || "9999-12-31"} ${a.start_time || "99:99"}`;
    const bKey = `${b.due || "9999-12-31"} ${b.start_time || "99:99"}`;
    return aKey.localeCompare(bKey);
  });
  if (!q) return list.slice(0, 30);
  return list.filter((task) => {
    const text = [
      task.title,
      task.description,
      task.due,
      formatTaskTime(task),
      priorityLabel(task.priority)
    ].join(" ").toLowerCase();
    return text.includes(q);
  }).slice(0, 30);
});
const quickMemoTemplate = computed(() => {
  const list = workspaceTemplates.value || [];
  return list.find((tpl) => tpl.key === quickMemoTemplateKey.value) || list[0] || null;
});
const quickMemoStatusText = computed(() => {
  if (quickMemoError.value) return quickMemoError.value;
  if (quickMemoState.value === "listening") {
    return quickMemoInterim.value ? `正在识别：${quickMemoInterim.value}` : "语音识别中...";
  }
  if (quickMemoState.value === "paused") return "语音识别已暂停，可以继续或生成笔记。";
  return "";
});
const calendarCommands = computed(() => {
  const commands = [
    { key: "create", label: "创建plan...", hint: "C" },
    { key: "goto-date", label: "跳转到日期...", hint: "." },
    { key: "today", label: "跳转今天", hint: "T" },
    { key: "align-today", label: "在视图中左对齐今天", hint: isMacLike.value ? "option T" : "alt T" },
    { key: "next-week", label: "跳转到下周", hint: "J" },
    { key: "prev-week", label: "跳转到上周", hint: "K" },
    { key: "search", label: "搜索活动", hint: "/" }
  ];
  const q = String(calendarCommandQuery.value || "").trim().toLowerCase();
  if (!q) return commands;
  return commands.filter((item) => item.label.toLowerCase().includes(q));
});

const workspaceStats = computed(() => {
  const stats = workspaceDashboard.value?.stats || {};
  return [
    { label: "笔记", value: stats.notes ?? activeNotes.value.length },
    { label: "未完成笔记", value: stats.unfinished_notes ?? contentNotes.value.filter((n) => n.status !== "completed").length },
    { label: "已完成笔记", value: stats.completed_notes ?? contentNotes.value.filter((n) => n.status === "completed").length },
    { label: "标签", value: allTags.value.length }
  ];
});

const dashboardNotePie = computed(() => workspaceDashboard.value?.note_status_pie || []);
const dashboardBars = computed(() => workspaceDashboard.value?.overview_bars || []);
const dashboardNoteTrend = computed(() => workspaceDashboard.value?.note_trend || []);

const cardStats = computed(() => {
  const cards = knowledgeCards.value || [];
  const active = cards.filter((card) => card.status === "active").length;
  const mastered = cards.filter((card) => card.status === "mastered").length;
  const archived = cards.filter((card) => card.status === "archived").length;
  return [
    { label: "全部卡片", value: cards.length },
    { label: "到期复习", value: dueKnowledgeCards.value.length },
    { label: "复习中", value: active },
    { label: "已掌握", value: mastered },
    { label: "已归档", value: archived }
  ];
});

const cardStatusPie = computed(() => [
  { label: "复习中", value: (knowledgeCards.value || []).filter((card) => card.status === "active").length },
  { label: "已掌握", value: (knowledgeCards.value || []).filter((card) => card.status === "mastered").length },
  { label: "已归档", value: (knowledgeCards.value || []).filter((card) => card.status === "archived").length }
]);

const cardReviewBars = computed(() => [
  { label: "今日到期", value: dueKnowledgeCards.value.length },
  {
    label: "本周到期",
    value: (knowledgeCards.value || []).filter((card) => {
      if (card.status !== "active" || !card.next_review_at) return false;
      const d = new Date(card.next_review_at);
      return !Number.isNaN(d.getTime()) && d <= new Date(Date.now() + 7 * 24 * 60 * 60 * 1000);
    }).length
  },
  { label: "已掌握", value: (knowledgeCards.value || []).filter((card) => card.status === "mastered").length },
  { label: "已归档", value: (knowledgeCards.value || []).filter((card) => card.status === "archived").length }
]);

const cardReviewTrend = computed(() => {
  const now = new Date();
  const points = [];
  for (let i = 13; i >= 0; i -= 1) {
    const d = new Date(now);
    d.setDate(now.getDate() - i);
    const key = `${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
    const value = (knowledgeCards.value || []).filter((card) => {
      if (!card.last_reviewed_at) return false;
      const reviewed = new Date(card.last_reviewed_at);
      if (Number.isNaN(reviewed.getTime())) return false;
      return reviewed.getMonth() === d.getMonth() && reviewed.getDate() === d.getDate() && reviewed.getFullYear() === d.getFullYear();
    }).length;
    points.push({ label: key, value });
  }
  return points;
});

const filteredCards = computed(() => {
  const q = String(cardSearch.value || "").trim().toLowerCase();
  return (knowledgeCards.value || []).filter((card) => {
    if (cardFilter.value !== "all" && card.status !== cardFilter.value) return false;
    if (!q) return true;
    return [card.front, card.back, ...(card.tags || [])].join(" ").toLowerCase().includes(q);
  });
});

const currentReviewCard = computed(() => {
  const list = (activeView.value === "cardReview" ? reviewSessionCards.value : dueKnowledgeCards.value) || [];
  if (!list.length) return null;
  const idx = Math.min(Math.max(0, cardReviewIndex.value), list.length - 1);
  return list[idx] || null;
});

const graphTags = computed(() => {
  const set = new Set();
  for (const node of workspaceGraph.value?.nodes || []) {
    for (const tag of node.tags || []) set.add(tag);
  }
  return Array.from(set).sort((a, b) => a.localeCompare(b, "zh-CN"));
});

const graphNodeMap = computed(() => {
  const map = new Map();
  for (const node of workspaceGraph.value?.nodes || []) map.set(Number(node.id), node);
  return map;
});

const graphSearchQuery = computed(() => String(graphQuery.value || "").trim().toLowerCase());

const graphSearchMatchedIDs = computed(() => {
  const q = graphSearchQuery.value;
  if (!q) return new Set();
  const matched = new Set();
  for (const node of workspaceGraph.value?.nodes || []) {
    if (graphTagFilter.value && !(node.tags || []).includes(graphTagFilter.value)) continue;
    if (graphNodeMatchesQuery(node, q)) matched.add(Number(node.id));
  }
  return matched;
});

const filteredGraphNodes = computed(() => {
  const nodes = workspaceGraph.value?.nodes || [];
  if (!graphSearchQuery.value) {
    return nodes.filter((node) => !graphTagFilter.value || (node.tags || []).includes(graphTagFilter.value));
  }
  const visible = new Set(graphSearchMatchedIDs.value);
  if (!visible.size) return [];
  for (const edge of workspaceGraph.value?.edges || []) {
    if (edge.type !== "link") continue;
    const source = Number(edge.source);
    const target = Number(edge.target);
    if (graphSearchMatchedIDs.value.has(source)) visible.add(target);
    if (graphSearchMatchedIDs.value.has(target)) visible.add(source);
  }
  return nodes.filter((node) => visible.has(Number(node.id)));
});

const filteredGraphEdges = computed(() => {
  const visible = new Set(filteredGraphNodes.value.map((node) => Number(node.id)));
  const matched = graphSearchMatchedIDs.value;
  return (workspaceGraph.value?.edges || []).filter((edge) => {
    const source = Number(edge.source);
    const target = Number(edge.target);
    if (!visible.has(source) || !visible.has(target)) return false;
    if (graphSearchQuery.value) {
      return edge.type === "link" && (matched.has(source) || matched.has(target));
    }
    if (!["all", "important"].includes(graphRelationFilter.value) && edge.type !== graphRelationFilter.value) return false;
    return true;
  });
});

const graphDegreeMap = computed(() => {
  const map = new Map(filteredGraphNodes.value.map((node) => [Number(node.id), 0]));
  for (const edge of filteredGraphEdges.value || []) {
    const score = graphEdgeStrength(edge) * 6;
    map.set(Number(edge.source), (map.get(Number(edge.source)) || 0) + score);
    map.set(Number(edge.target), (map.get(Number(edge.target)) || 0) + score);
  }
  return map;
});

const graphTopicSummary = computed(() => {
  const groups = new Map();
  for (const node of filteredGraphNodes.value || []) {
    const key = graphPrimaryTag(node, graphTagFilter.value);
    const current = groups.get(key) || { label: key, count: 0, degree: 0, color: graphClusterColor(key) };
    current.count += 1;
    current.degree += graphDegreeMap.value.get(Number(node.id)) || 0;
    groups.set(key, current);
  }
  return Array.from(groups.values())
    .sort((a, b) => b.count - a.count || b.degree - a.degree)
    .slice(0, 10);
});

const graphHubNodes = computed(() => {
  return [...(filteredGraphNodes.value || [])]
    .map((node) => ({ ...node, degree: graphDegreeMap.value.get(Number(node.id)) || 0 }))
    .sort((a, b) => Number(b.degree || 0) - Number(a.degree || 0))
    .slice(0, 12);
});

const graphFocusedNode = computed(() => {
  const id = Number(graphFocusID.value || 0);
  return id ? graphNodeMap.value.get(id) || null : null;
});

const graphVisibleNodes = computed(() => {
  const nodes = filteredGraphNodes.value || [];
  const focusID = Number(graphFocusID.value || 0);
  if (!focusID) return nodes;
  const visible = new Set([focusID]);
  for (const edge of filteredGraphEdges.value || []) {
    if (Number(edge.source) === focusID) visible.add(Number(edge.target));
    if (Number(edge.target) === focusID) visible.add(Number(edge.source));
  }
  return nodes.filter((node) => visible.has(Number(node.id)));
});

const graphVisibleEdges = computed(() => {
  const visible = new Set(graphVisibleNodes.value.map((node) => Number(node.id)));
  const focusID = Number(graphFocusID.value || 0);
  let edges = (filteredGraphEdges.value || []).filter((edge) => visible.has(Number(edge.source)) && visible.has(Number(edge.target)));
  if (focusID) {
    edges = edges.filter((edge) => Number(edge.source) === focusID || Number(edge.target) === focusID);
  } else if (graphSearchQuery.value) {
    edges = [...edges].sort(compareGraphEdges);
  } else if (graphRelationFilter.value === "important") {
    edges = pickReadableGraphEdges(edges, 32, 2);
  } else if (graphRelationFilter.value === "all") {
    edges = edges.filter((edge) => edge.type === "link" || Number(edge.weight || 0) >= 0.7);
  }
  const nodeCount = graphVisibleNodes.value.length;
  const maxEdges = graphSearchQuery.value
    ? Math.max(40, Math.min(120, nodeCount * 6))
    : focusID
    ? Math.max(24, Math.min(72, nodeCount * 3))
    : Math.max(28, Math.min(48, nodeCount));
  if (edges.length <= maxEdges) return edges;
  return [...edges].sort(compareGraphEdges).slice(0, maxEdges);
});

const graphLayoutNodes = computed(() => layoutGraphNodes(graphVisibleNodes.value, graphVisibleEdges.value, graphTagFilter.value, graphFocusID.value));
const graphLayoutEdges = computed(() => {
  const positions = new Map(graphLayoutNodes.value.map((node) => [Number(node.id), node]));
  return graphVisibleEdges.value
    .map((edge) => ({
      ...edge,
      searchDirection: graphSearchQuery.value ? graphSearchEdgeDirection(edge, graphSearchMatchedIDs.value) : "",
      sourceNode: positions.get(Number(edge.source)),
      targetNode: positions.get(Number(edge.target))
    }))
    .filter((edge) => edge.sourceNode && edge.targetNode);
});

const graphClusterLabels = computed(() => {
  const clusters = new Map();
  for (const node of graphLayoutNodes.value || []) {
    const key = node.cluster || "未分类";
    const current = clusters.get(key) || { label: key, count: 0, x: 0, y: 0 };
    current.count += 1;
    current.x += Number(node.x || 0);
    current.y += Number(node.y || 0);
    clusters.set(key, current);
  }
  return Array.from(clusters.values())
    .map((item) => ({
      ...item,
      x: item.x / item.count,
      y: item.y / item.count
    }))
    .filter((item) => item.count >= 2)
    .sort((a, b) => b.count - a.count)
    .slice(0, 8);
});

const researchHistoryItems = computed(() => {
  return [...(researchHistory.value || [])]
    .sort((a, b) => Number(b.createdAt || 0) - Number(a.createdAt || 0))
    .slice(0, 30);
});

const recommendHistoryItems = computed(() => {
  return [...(recommendHistory.value || [])]
    .sort((a, b) => Number(b.createdAt || 0) - Number(a.createdAt || 0))
    .slice(0, 30);
});

const qualityHubStats = computed(() => {
  const notes = contentNotes.value || [];
  const withoutTags = notes.filter((note) => !(note.tags || []).length).length;
  const unfinishedNotes = notes.filter((note) => note.status !== "completed").length;
  return [
    { label: "可整理笔记", value: notes.length },
    { label: "未完成笔记", value: unfinishedNotes },
    { label: "缺少标签", value: withoutTags }
  ];
});

const localQualityHubItems = computed(() => {
  return (contentNotes.value || [])
    .map((note) => {
      const issues = [];
      if (note.status !== "completed") issues.push("未完成笔记");
      if (!(note.tags || []).length) issues.push("缺少标签");
      if (!note.parent_id) issues.push("建议归入文件夹");
      if (plainNoteText(note).length < 120) issues.push("正文偏短");
      if (!String(note.markdown || "").includes("##")) issues.push("缺少二级结构");
      const summary = issues.length ? `初筛发现 ${issues.slice(0, 2).join("、")}，建议进一步确认内容完整度。` : "";
      return {
        note,
        issues,
        score: Math.max(35, 100 - issues.length * 12),
        summary,
        action: issues.includes("未完成笔记") ? "补齐下一步内容后，将状态改为已完成。" : "补充标签、结构或归档位置。",
        source: "local"
      };
    })
    .filter((item) => item.issues.length)
    .sort((a, b) => a.score - b.score)
    .slice(0, 8);
});

const qualityHubItems = computed(() => localQualityHubItems.value);
const qualityUnfinishedNotes = computed(() => {
  return (contentNotes.value || [])
    .filter((note) => note.status !== "completed")
    .sort((a, b) => new Date(b.updated_at || 0) - new Date(a.updated_at || 0));
});

const writingBriefs = computed(() => [
  {
    title: "把一篇短笔记扩展成教程",
    desc: "选择内容偏短的笔记，补充背景、步骤、示例和延伸阅读。",
    target: qualityHubItems.value[0]?.note || contentNotes.value[0]
  },
  {
    title: "生成一个答辩演示脚本",
    desc: "围绕产品与规划、AI 与检索、前端体验组织一条讲述路线。",
    target: (contentNotes.value || []).find((note) => String(note.title || "").includes("路线图"))
  },
  {
    title: "整理本周学习周报",
    desc: "从任务中心和最近更新笔记中提炼完成项与下周计划。",
    target: (contentNotes.value || []).find((note) => String(note.title || "").includes("回顾")) || contentNotes.value[0]
  }
]);

const recommendSelectedNotes = computed(() => {
  const byID = notesByID.value;
  return (recommendSelectedIDs.value || [])
    .map((id) => byID.get(Number(id)))
    .filter(Boolean);
});

const weeklySelectedNotes = computed(() => {
  const byID = notesByID.value;
  return (weeklySelectedIDs.value || [])
    .map((id) => byID.get(Number(id)))
    .filter(Boolean);
});

const filteredRecommendNotes = computed(() => {
  const q = String(recommendNoteSearch.value || "").trim().toLowerCase();
  const list = (activeNotes.value || []).filter((item) => !isFolder(item));
  if (!q) return list;
  return list.filter((note) => {
    const haystack = [
      noteFullPath(note),
      note.title,
      ...(note.tags || [])
    ].join(" ").toLowerCase();
    return haystack.includes(q);
  });
});

const filteredWeeklyNotes = computed(() => {
  const q = String(weeklyNoteSearch.value || "").trim().toLowerCase();
  const list = (activeNotes.value || []).filter((item) => !isFolder(item));
  if (!q) return list;
  return list.filter((note) => {
    const haystack = [
      noteFullPath(note),
      note.title,
      ...(note.tags || [])
    ].join(" ").toLowerCase();
    return haystack.includes(q);
  });
});

const filteredAssistantNotes = computed(() => {
  const q = String(assistantNoteSearch.value || "").trim().toLowerCase();
  const selected = new Set(
    (assistantAttachments.value || [])
      .filter((item) => item.type === "note")
      .map((item) => Number(item.id))
  );
  const list = (activeNotes.value || []).filter((item) => !isFolder(item) && !selected.has(Number(item.id)));
  if (!q) return list.slice(0, 30);
  return list.filter((note) => {
    const haystack = [
      noteFullPath(note),
      note.title,
      ...(note.tags || [])
    ].join(" ").toLowerCase();
    return haystack.includes(q);
  }).slice(0, 30);
});

const insightRecommendations = computed(() => noteInsights.value?.recommendations || []);
const insightLinks = computed(() => noteInsights.value?.links || { outgoing: [], backlinks: [], unlinked_mentions: [] });
const insightFlashcards = computed(() => noteInsights.value?.flashcards || []);
const insightSuggestedTags = computed(() => noteInsights.value?.suggested_tags || []);
const insightQualityIssues = computed(() => noteInsights.value?.quality_issues || []);
const insightDuplicates = computed(() => noteInsights.value?.duplicate_warnings || []);
const insightOutline = computed(() => noteInsights.value?.outline || []);

const guideMap = {
  workspace: {
    title: "智能知识工作台",
    subtitle: "这里是工具导航，不承载具体工作。点击卡片进入独立页面处理推荐、任务、模板和导入。",
    visual: "workspace",
    steps: ["从侧边栏进入智能知识工作台。", "选择你要使用的工具卡片。", "进入独立页面后再完成具体操作。"],
    tips: ["工作台本身只负责导航，让主页保持轻盈。"]
  },
  recommend: {
    title: "内容推荐",
    subtitle: "输入希望被推荐的主题，也可以选择自己的笔记作为参考，让 AI 给出联网方向、笔记关联和总结。",
    visual: "recommend",
    steps: ["在搜索栏输入你想研究或被推荐的主题。", "点击文件按钮选择一篇或多篇笔记作为参考。", "点击生成推荐后，查看 AI 总结、推荐方向和关联笔记。"],
    tips: ["如果当前模型或服务没有联网能力，系统会基于模型知识和你的笔记内容给出最接近的推荐。"]
  },
  tasks: {
    title: "任务中心",
    subtitle: "像 Todo List 一样管理个人计划，也能参考笔记中扫描到的待办。",
    visual: "tasks",
    steps: ["写下计划并选择优先级和日期。", "点击添加后进入任务列表。", "勾选、删除或切换筛选器来管理计划。"],
    tips: ["个人计划保存在当前浏览器；页面也会展示从笔记 Markdown 扫描出的待办。"]
  },
  templates: {
    title: "模板库",
    subtitle: "用预设模板快速创建笔记，也可以新增、编辑或删除自己的模板。",
    visual: "templates",
    steps: ["点击模板卡片创建新笔记。", "点击新增模板保存自己的结构。", "编辑模板时通过下拉多选已有标签，也可以新增标签。"],
    tips: ["自定义模板保存在当前浏览器；内置模板被编辑后会以自定义版本展示。"]
  },
  import: {
    title: "文档导入",
    subtitle: "从电脑选择 Markdown 或文本文件，并指定目标文件夹。",
    visual: "import",
    steps: ["点击选择本地文件。", "选择目标文件夹，也可以选择根目录。", "确认后系统会创建笔记并建立索引。"],
    tips: ["适合导入 .md、.txt、.csv、.json 等文本类文件。"]
  },
  insight: {
    title: "笔记智能洞察",
    subtitle: "在阅读页侧栏展示摘要、质量评分、链接、推荐和已保存的复习问题。",
    visual: "insight",
    steps: ["打开任意笔记进入阅读页。", "在右侧查看智能洞察和质量分。", "点击刷新可重新计算当前笔记洞察。"],
    tips: ["洞察只会在你点击刷新时重新计算。"]
  },
  tags: {
    title: "智能标签",
    subtitle: "从正文关键词和已有标签中推荐适合当前笔记的标签。",
    visual: "tags",
    steps: ["在笔记侧栏查看建议标签。", "点击 + 标签加入当前笔记。", "系统会切换到编辑状态，保存后生效。"],
    tips: ["建议标签会避开已经存在的标签。"]
  },
  backlinks: {
    title: "双向链接",
    subtitle: "展示当前笔记链接到哪里、被谁引用，以及正文提到但未链接的页面。",
    visual: "links",
    steps: ["查看反向链接找到引用当前页的笔记。", "查看未链接提及，发现可以补链的页面。", "点击条目跳转查看上下文。"],
    tips: ["用 [[页面标题]] 可以快速建立内部链接。"]
  },
  flashcards: {
    title: "复习问题",
    subtitle: "自己创建问题，或主动让 AI 生成问题后再编辑沉淀。",
    visual: "cards",
    steps: ["点击手动添加写自己的复习问题。", "点击 AI 生成获取问题思路。", "保存后的问题可以编辑、删除，也可以一键放入 AI 助手复习。"],
    tips: ["AI 生成只会在你主动点击后发生。"]
  },
  quality: {
    title: "质量检查",
    subtitle: "检查标题、内容长度、结构、标签和链接完整度。",
    visual: "quality",
    steps: ["查看质量分和问题列表。", "根据建议补充标题、标签、大纲或链接。", "保存后刷新洞察确认改善。"],
    tips: ["质量分不是绝对评分，只用于提示整理方向。"]
  },
  qualityHub: {
    title: "知识体检中心",
    subtitle: "汇总全库未完成状态，方便集中回到还没收尾的笔记。",
    visual: "quality",
    steps: ["进入知识体检中心查看全库指标。", "从未完成列表打开笔记。", "补齐内容后把状态标记为已完成。"],
    tips: ["适合在大量导入后做一次集中收尾。"]
  },
  writingStudio: {
    title: "写作中心",
    subtitle: "调用 AI 根据本周笔记或本机文件生成学习周报，并创建到指定文件夹。",
    visual: "templates",
    steps: ["填写周报标题。", "选择生成后的目标文件夹。", "可选：选择笔记库笔记或本机文本文件作为来源。", "点击生成后系统会创建一篇真实笔记。"],
    tips: ["周报会包含本周学习大纲、下周学习建议和资源推荐。"]
  }
};

const currentGuide = computed(() => guideMap[currentGuideKey.value] || guideMap.workspace);

const notePathMap = computed(() => {
  const out = new Map();
  for (const note of activeNotes.value || []) {
    const path = normalizeNotePath(noteFullPath(note));
    if (!path) continue;
    out.set(path, note.id);
  }
  return out;
});

const noteNavChain = computed(() => {
  if (!selectedNote.value?.id) return [];
  const byID = notesByID.value;
  const chain = [];
  let cur = selectedNote.value;
  const seen = new Set();
  while (cur && !seen.has(cur.id)) {
    seen.add(cur.id);
    chain.push(cur);
    cur = cur.parent_id ? byID.get(cur.parent_id) || null : null;
  }
  return chain.reverse();
});

const selectedFolder = computed(() => {
  const id = Number(selectedFolderID.value || 0);
  if (!id) return null;
  const n = notesByID.value.get(id) || null;
  if (!n || !isFolder(n)) return null;
  return n;
});

const greeting = computed(() => {
  const h = new Date().getHours();
  if (h < 6) return "凌晨好";
  if (h < 12) return "上午好";
  if (h < 18) return "下午好";
  return "晚上好";
});

const saveLabel = computed(() => {
  if (!selectedId.value) return "新页面";
  switch (saveState.value) {
    case "saving":
      return "自动保存中...";
    case "dirty":
      return "未保存";
    case "saved":
      return "已保存";
    case "error":
      return "自动保存失败";
    default:
      return "空闲";
  }
});

const recentCards = computed(() => {
  const out = [];
  const byID = notesByID.value;
  for (const item of recentVisits.value || []) {
    const note = byID.get(item.id);
    if (!note || note.is_archived) continue;
    out.push({ note, visitedAt: item.at });
  }
  if (out.length > 0) return out;
  return (activeNotes.value || []).slice(0, 12).map((n) => ({ note: n, visitedAt: n.updated_at }));
});

const favoriteSet = computed(() => new Set(favoriteIDs.value || []));
const hasFavorite = computed(() => favoriteSet.value.has(Number(selectedId.value || 0)));
const voiceSupported = computed(() => !!speechRecognitionCtor());

const favoriteCards = computed(() => {
  const out = [];
  const byID = notesByID.value;
  for (const id of favoriteIDs.value || []) {
    const note = byID.get(id);
    if (!note || note.is_archived) continue;
    out.push({ note, favoritedAt: note.updated_at });
  }
  return out;
});

const contextNote = computed(() => {
  const id = Number(contextNoteID.value || 0);
  if (!id) return null;
  return notesByID.value.get(id) || null;
});

const selectedTagSet = computed(() => {
  const set = new Set();
  for (const t of selectedTags.value || []) {
    const key = String(t || "").trim().toLowerCase();
    if (!key) continue;
    set.add(key);
  }
  return set;
});

const tagQueryTrimmed = computed(() => String(tagQuery.value || "").trim());

const filteredTagOptions = computed(() => {
  const q = tagQueryTrimmed.value.toLowerCase();
  const list = allTags.value || [];
  if (!q) return list;
  return list.filter((t) => String(t || "").toLowerCase().includes(q));
});

const canCreateTag = computed(() => {
  const t = tagQueryTrimmed.value;
  if (!t) return false;
  return !selectedTagSet.value.has(t.toLowerCase()) && !(allTags.value || []).some((x) => String(x).toLowerCase() === t.toLowerCase());
});

const privateRows = computed(() => {
  const list = activeNotes.value || [];
  const byID = new Map(list.map((n) => [n.id, n]));
  const children = new Map();
  for (const n of list) {
    const parent = n.parent_id ? byID.get(n.parent_id) || null : null;
    const pid = parent && isFolder(parent) ? parent.id : 0;
    if (!children.has(pid)) children.set(pid, []);
    children.get(pid).push(n);
  }
  for (const arr of children.values()) {
    arr.sort((a, b) => String(b.updated_at).localeCompare(String(a.updated_at)));
  }

  const roots = list.filter((n) => {
    if (!n.parent_id) return true;
    const parent = byID.get(n.parent_id) || null;
    return !parent || !isFolder(parent);
  });
  roots.sort((a, b) => String(b.updated_at).localeCompare(String(a.updated_at)));

  const rows = [];
  const walk = (note, depth) => {
    const kids = children.get(note.id) || [];
    rows.push({
      note,
      depth,
      hasChildren: kids.length > 0,
      expanded: folderOpen.value[note.id] !== false
    });
    if (kids.length > 0 && folderOpen.value[note.id] !== false) {
      for (const child of kids) walk(child, depth + 1);
    }
  };
  for (const root of roots) walk(root, 0);
  return rows;
});

const filteredTrash = computed(() => {
  const q = trashQuery.value.trim().toLowerCase();
  if (!q) return archivedNotes.value || [];
  return (archivedNotes.value || []).filter((n) => String(n.title || "").toLowerCase().includes(q));
});

const searchSortLabel = computed(() => {
  if (searchSort.value === "updated_desc") return "最近编辑";
  if (searchSort.value === "updated_asc") return "最早编辑";
  if (searchSort.value === "title_asc") return "标题顺序";
  return "相关度";
});

const searchDateLabel = computed(() => {
  if (searchDate.value === "today") return "今天";
  if (searchDate.value === "30d") return "过去30天";
  if (searchDate.value === "older") return "更早";
  return "全部";
});

const searchTagLabel = computed(() => {
  const list = normalizeTags(searchTags.value || []);
  if (list.length === 0) return "标签：全部";
  if (list.length === 1) return `标签：${list[0]}`;
  return `标签：已选 ${list.length} 个`;
});
const searchTagOptions = computed(() => normalizeTags(allTags.value || []));
const aiThreadKey = computed(() => {
  if (activeView.value === "assistant") return assistantThreadKey.value;
  const id = Number(selectedId.value || 0);
  return activeView.value === "note" && id > 0 ? `note:${id}` : "home";
});

const assistantHistoryItems = computed(() => {
  return Array.from(aiThreads.value.entries())
    .filter(([key, entry]) => String(key).startsWith("assistant:") && (entry?.messages || []).length > 0)
    .sort((a, b) => Number(b[1]?.updatedAt || 0) - Number(a[1]?.updatedAt || 0))
    .map(([key, entry]) => {
      const messages = entry.messages || [];
      const firstUser = messages.find((msg) => msg.role === "user")?.content || "新会话";
      const lastMessage = [...messages].reverse().find((msg) => msg.content)?.content || firstUser;
      return {
        key,
        title: summarizeLine(firstUser, 22),
        preview: summarizeLine(lastMessage, 46),
        updatedAt: formatRecentDate(entry.updatedAt)
      };
    });
});

const assistantPlaceholder = computed(() => {
  if (assistantScope.value === "workspace") {
    return "整理计划、处理附件、把临时材料转成下一步行动...";
  }
  return "向全库笔记提问，例如：RAG 流程怎么实现？哪些笔记提到检索评估？";
});

const assistantScopeText = computed(() => {
  if (assistantAttachmentReading.value) return "读取中";
  if (assistantAttachments.value.length) {
    return assistantScope.value === "library"
      ? `全库 + ${assistantAttachments.value.length} 个附件`
      : `工作台 + ${assistantAttachments.value.length} 个附件`;
  }
  return assistantScope.value === "library" ? "全库问答" : "工作台";
});

const filteredSearchResults = computed(() => {
  const q = searchText.value.trim();
  let list = (searchResults.value || []).map((n) => ({
    note: n,
    score: scoreSearchResult(n, q),
    titleMatch: containsFold(String(n.title || ""), q),
    bodyMatch: containsFold(String(n.markdown || ""), q)
  }));

  if (q) {
    list = list.filter((item) => {
      if (searchOnlyTitle.value) return item.titleMatch;
      if (searchInPage.value) return item.bodyMatch;
      return item.titleMatch || item.bodyMatch;
    });
  }

  const wantedTags = normalizeTags(searchTags.value || []).map((tag) => String(tag).toLowerCase());
  if (wantedTags.length > 0) {
    list = list.filter((item) => {
      const noteTags = new Set((item.note.tags || []).map((tag) => String(tag).toLowerCase()));
      return wantedTags.every((tag) => noteTags.has(tag));
    });
  }

  if (searchDate.value !== "all") {
    list = list.filter((item) => timeBucket(item.note.updated_at) === searchDate.value);
  }

  const byUpdatedDesc = (a, b) => String(b.note.updated_at).localeCompare(String(a.note.updated_at));
  if (searchSort.value === "updated_desc") {
    list.sort(byUpdatedDesc);
  } else if (searchSort.value === "updated_asc") {
    list.sort((a, b) => String(a.note.updated_at).localeCompare(String(b.note.updated_at)));
  } else if (searchSort.value === "title_asc") {
    list.sort((a, b) => String(a.note.title || "").localeCompare(String(b.note.title || ""), "zh-CN"));
  } else {
    list.sort((a, b) => {
      if (b.score !== a.score) return b.score - a.score;
      return byUpdatedDesc(a, b);
    });
  }
  return list.map((item) => item.note);
});

const searchGroups = computed(() => {
  const groups = new Map();
  for (const n of filteredSearchResults.value || []) {
    const key = timeBucket(n.updated_at);
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(n);
  }
  return [
    { key: "today", label: "今天", items: groups.get("today") || [] },
    { key: "30d", label: "过去 30 天", items: groups.get("30d") || [] },
    { key: "older", label: "更早", items: groups.get("older") || [] }
  ].filter((g) => g.items.length > 0);
});

const flatSearchResults = computed(() => {
  return (searchGroups.value || []).flatMap((g) => g.items || []);
});

const searchActiveNoteID = computed(() => {
  const idx = Number(searchCursor.value);
  const list = flatSearchResults.value || [];
  if (idx < 0 || idx >= list.length) return 0;
  return Number(list[idx]?.id || 0);
});

const voiceStatusText = computed(() => {
  if (voiceError.value) return voiceError.value;
  if (voiceState.value === "listening") {
    return voiceInterim.value ? `正在识别：${voiceInterim.value}` : "语音输入中...";
  }
  if (voiceState.value === "paused") {
    return voiceTranscript.value ? "语音输入已暂停，可以继续或结束插入。" : "语音输入已暂停。";
  }
  return "";
});

async function api(path, options = {}) {
  const res = await fetch(path, {
    headers: { "Content-Type": "application/json" },
    ...options
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || `请求失败：${res.status}`);
  return data;
}

async function loadHomeIntelligence() {
  homeIntelligenceLoading.value = true;
  try {
    const [review, tasks, tpl, dashboard] = await Promise.all([
      api("/api/review"),
      api("/api/tasks"),
      api("/api/templates"),
      api("/api/workspace/dashboard")
    ]);
    reviewReport.value = review;
    taskItems.value = Array.isArray(tasks) ? tasks : [];
    templates.value = Array.isArray(tpl) ? tpl : [];
    workspaceDashboard.value = dashboard;
  } catch {
    reviewReport.value = reviewReport.value || null;
    taskItems.value = taskItems.value || [];
    templates.value = templates.value || [];
  } finally {
    homeIntelligenceLoading.value = false;
  }
}

async function loadWorkspaceDashboard() {
  workspaceDashboardLoading.value = true;
  try {
    workspaceDashboard.value = await api("/api/workspace/dashboard");
  } finally {
    workspaceDashboardLoading.value = false;
  }
}

async function loadKnowledgeCards() {
  const [cards, due] = await Promise.all([
    api("/api/cards?include_archived=1"),
    api("/api/cards/review/due")
  ]);
  knowledgeCards.value = Array.isArray(cards) ? cards : [];
  dueKnowledgeCards.value = Array.isArray(due) ? due : [];
  if (cardReviewIndex.value >= dueKnowledgeCards.value.length) cardReviewIndex.value = 0;
}

async function loadWorkspaceGraph() {
  graphLoading.value = true;
  try {
    const graph = await api("/api/workspace/graph?limit=80");
    if (Array.isArray(graph?.nodes) && graph.nodes.length > 0 && Array.isArray(graph?.edges) && graph.edges.length > 0) {
      workspaceGraph.value = graph;
      return;
    }
    workspaceGraph.value = await buildLocalWorkspaceGraph();
  } catch {
    workspaceGraph.value = await buildLocalWorkspaceGraph();
  } finally {
    graphLoading.value = false;
  }
}

async function buildLocalWorkspaceGraph() {
  let sourceNotes = [];
  try {
    sourceNotes = await api("/api/notes?include_archived=1");
  } catch {
    sourceNotes = allNotes.value || [];
  }
  const notes = (sourceNotes || [])
    .filter((note) => !note.is_archived && !isFolder(note))
    .slice()
    .sort((a, b) => new Date(b.updated_at || 0) - new Date(a.updated_at || 0))
    .slice(0, 80);
  const byID = new Map(notes.map((note) => [Number(note.id), note]));
  const byTitle = new Map();
  for (const note of notes) {
    const title = String(note.title || "").trim().toLowerCase();
    if (title) byTitle.set(title, Number(note.id));
  }
  const nodes = notes.map((note) => ({
    id: Number(note.id),
    title: note.title || "未命名",
    path: noteFullPathFromMap(note, byID),
    tags: note.tags || [],
    quality_score: localGraphQualityScore(note),
    updated_at: note.updated_at
  }));
  const edges = localGraphEdges(notes, byTitle);
  return { nodes, edges };
}

function localGraphEdges(notes, byTitle) {
  const edges = new Map();
  const addEdge = (source, target, type, weight, reason) => {
    const a = Number(source);
    const b = Number(target);
    if (!a || !b || a === b) return;
    const key = type === "link"
      ? `${a}:${b}:${type}`
      : `${Math.min(a, b)}:${Math.max(a, b)}:${type}`;
    const current = edges.get(key);
    if (current && Number(current.weight || 0) >= weight) return;
    edges.set(key, { source: a, target: b, type, weight: Math.round(weight * 100) / 100, reason });
  };

  for (const note of notes) {
    for (const target of localLinkedNoteIDs(note, byTitle)) {
      addEdge(note.id, target, "link", 1, "内部链接");
    }
  }
  for (let i = 0; i < notes.length; i += 1) {
    for (let j = i + 1; j < notes.length; j += 1) {
      const a = notes[i];
      const b = notes[j];
      const sharedTags = (a.tags || []).filter((tag) => (b.tags || []).includes(tag)).length;
      if (sharedTags > 0) addEdge(a.id, b.id, "tag", Math.min(0.35 + sharedTags * 0.2, 0.95), "共享标签");
      const similarity = localTokenSimilarity(`${a.title || ""} ${a.markdown || ""}`, `${b.title || ""} ${b.markdown || ""}`);
      if (similarity >= 0.12) addEdge(a.id, b.id, "similar", similarity, "内容相似");
    }
  }
  return Array.from(edges.values()).sort((a, b) => Number(b.weight || 0) - Number(a.weight || 0));
}

function localLinkedNoteIDs(note, byTitle) {
  const out = new Set();
  const md = String(note.markdown || "");
  for (const match of md.matchAll(/note:\/\/(\d+)/g)) {
    const id = Number(match[1]);
    if (id) out.add(id);
  }
  for (const match of md.matchAll(/\[\[([^\]]+)\]\]/g)) {
    const id = byTitle.get(String(match[1] || "").trim().toLowerCase());
    if (id) out.add(id);
  }
  for (const match of md.matchAll(/\[[^\]]+\]\(([^)]+)\)/g)) {
    const target = String(match[1] || "").trim();
    if (target.startsWith("note://")) {
      const id = Number(target.replace("note://", ""));
      if (id) out.add(id);
    }
  }
  return out;
}

function localGraphQualityScore(note) {
  let score = 100;
  const text = String(note.markdown || "").trim();
  if (!text) score -= 35;
  if (!(note.tags || []).length) score -= 18;
  if (!text.includes("##")) score -= 12;
  if (text.length < 160) score -= 18;
  return Math.max(35, score);
}

function noteFullPathFromMap(note, byID) {
  const parts = [];
  let cur = note;
  const seen = new Set();
  while (cur && !seen.has(Number(cur.id))) {
    seen.add(Number(cur.id));
    parts.push(cur.title || "未命名");
    cur = cur.parent_id ? byID.get(Number(cur.parent_id)) : null;
  }
  return parts.reverse().join("/");
}

function localTokenSimilarity(a, b) {
  const left = localTokenSet(a);
  const right = localTokenSet(b);
  if (!left.size || !right.size) return 0;
  let shared = 0;
  for (const token of left) {
    if (right.has(token)) shared += 1;
  }
  return shared / (left.size + right.size - shared);
}

function localTokenSet(text) {
  const matches = String(text || "").toLowerCase().match(/[\u4e00-\u9fa5]{2,}|[a-z0-9_+.-]+/g) || [];
  return new Set(matches.filter((token) => token.length >= 2).slice(0, 120));
}

async function ensureWorkspaceGraph() {
  if (workspaceGraph.value?.nodes?.length) return;
  await loadWorkspaceGraph();
}

async function loadNoteIntelligence(id = selectedId.value, options = {}) {
  const { clear = false, message = "" } = options;
  const num = Number(id || 0);
  if (!num) {
    noteInsights.value = null;
    intelligenceMessage.value = "";
    return;
  }
  const token = intelligenceRequestToken.value + 1;
  intelligenceRequestToken.value = token;
  if (clear) noteInsights.value = null;
  intelligenceMessage.value = message;
  intelligenceLoading.value = true;
  try {
    const insights = await api(`/api/notes/${num}/insights`, { cache: "no-store" });
    if (token !== intelligenceRequestToken.value || Number(selectedId.value || 0) !== num) return;
    noteInsights.value = insights;
    intelligenceMessage.value = "";
  } catch {
    if (token !== intelligenceRequestToken.value || Number(selectedId.value || 0) !== num) return;
    noteInsights.value = null;
    intelligenceMessage.value = "刷新失败，请稍后再试。";
  } finally {
    if (token === intelligenceRequestToken.value) {
      intelligenceLoading.value = false;
    }
  }
}

async function loadCachedNoteIntelligence(id = selectedId.value) {
  const num = Number(id || 0);
  if (!num) return;
  const token = intelligenceRequestToken.value + 1;
  intelligenceRequestToken.value = token;
  intelligenceLoading.value = false;
  try {
    const insights = await api(`/api/notes/${num}/insights?cached=1`);
    if (token !== intelligenceRequestToken.value || Number(selectedId.value || 0) !== num) return;
    noteInsights.value = insights;
    intelligenceMessage.value = "";
  } catch {
    if (token !== intelligenceRequestToken.value || Number(selectedId.value || 0) !== num) return;
    noteInsights.value = null;
    intelligenceMessage.value = "";
  }
}

async function refreshNoteIntelligence() {
  if (!selectedId.value || intelligenceLoading.value) return;
  clearTimeout(autosaveTimer.value);
  if (isSaving.value) {
    intelligenceMessage.value = "正在保存后刷新...";
    for (let i = 0; i < 60 && isSaving.value; i += 1) {
      await delay(80);
    }
    if (isSaving.value) {
      intelligenceMessage.value = "保存还在进行，稍后再刷新。";
      return;
    }
  }
  const sig = payloadSignature(buildPayload());
  if (sig !== lastSavedSignature.value) {
    intelligenceMessage.value = "正在保存后刷新...";
    try {
      const saved = await saveNote(true);
      if (!saved) return;
    } catch {
      intelligenceMessage.value = "保存失败，暂时无法刷新洞察。";
      return;
    }
  }
  await loadNoteIntelligence(selectedId.value, { clear: true, message: "正在刷新笔记洞察..." });
}

function delay(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function resetImportToolState() {
  importTitle.value = "";
  importText.value = "";
  importTags.value = [];
  importTagMenuOpen.value = false;
  importTagQuery.value = "";
  importFileName.value = "";
  importParentID.value = "";
  importReading.value = false;
  if (importFileInputRef.value) importFileInputRef.value.value = "";
}

function resetRecommendToolState(options = {}) {
  recommendQuery.value = String(options.recommendQuery || "");
  recommendSelectedIDs.value = Array.isArray(options.recommendSelectedIDs) ? [...options.recommendSelectedIDs] : [];
  recommendNotePickerOpen.value = false;
  recommendNoteSearch.value = "";
  recommendLoading.value = false;
  recommendResult.value = null;
  recommendHistoryOpen.value = false;
}

function resetResearchToolState(options = {}) {
  researchTopic.value = String(options.researchTopic || "");
  researchLoading.value = false;
  researchResult.value = null;
  researchCardCreating.value = "";
  researchHistoryOpen.value = false;
}

function resetWeeklyReportState() {
  weeklyReportTitle.value = defaultWeeklyReportTitle();
  weeklyReportParentID.value = "";
  weeklyReportLoading.value = false;
  weeklyReportResult.value = null;
  weeklyNotePickerOpen.value = false;
  weeklyNoteSearch.value = "";
  weeklySelectedIDs.value = [];
  weeklyLocalFiles.value = [];
  weeklyFileReading.value = false;
  if (weeklyFileInputRef.value) weeklyFileInputRef.value.value = "";
}

function resetQuickMemoState() {
  try {
    quickMemoRecognitionRef.value?.abort?.();
  } catch {}
  quickMemoTemplateKey.value = workspaceTemplates.value?.[0]?.key || "";
  quickMemoParentID.value = "";
  quickMemoText.value = "";
  quickMemoInterim.value = "";
  quickMemoState.value = "idle";
  quickMemoError.value = "";
  quickMemoSaving.value = false;
  quickMemoResult.value = null;
  quickMemoFinishPending.value = false;
}

function resetWritingToolState() {
  resetWeeklyReportState();
  resetQuickMemoState();
}

function resetTaskToolState() {
  planTaskTitle.value = "";
  planTaskDate.value = "";
  planTaskStartTime.value = "09:00";
  planTaskEndTime.value = "10:00";
  planTaskPriority.value = "medium";
  planTaskFilter.value = "open";
}

function resetCalendarToolState() {
  const today = localDateKey(new Date());
  calendarSelectedDate.value = today;
  calendarSelectedTaskID.value = null;
  calendarCommandOpen.value = false;
  calendarCommandQuery.value = "";
  calendarCommandCursor.value = 0;
  calendarPlanSearchOpen.value = false;
  calendarPlanSearchQuery.value = "";
  calendarPlanSearchCursor.value = 0;
  calendarContextMenuOpen.value = false;
  calendarContextDate.value = "";
  calendarContextHour.value = 9;
  planTaskDate.value = today;
  planTaskStartTime.value = "09:00";
  planTaskEndTime.value = "10:00";
  planTaskPriority.value = "medium";
}

function resetCardToolState() {
  cardSearch.value = "";
  cardFilter.value = "active";
  closeCardForm();
}

function resetKnowledgeGraphToolState() {
  graphRelationFilter.value = "important";
  graphTagFilter.value = "";
  graphQuery.value = "";
  graphFocusID.value = 0;
}

function resetTemplateToolState() {
  closeTemplateModal();
}

function resetToolViewState(view, options = {}) {
  if (view === "tasks") resetTaskToolState();
  if (view === "templates") resetTemplateToolState();
  if (view === "import") resetImportToolState();
  if (view === "recommend") resetRecommendToolState(options);
  if (view === "researchStudio") resetResearchToolState(options);
  if (view === "writingStudio") resetWritingToolState();
  if (view === "calendar") resetCalendarToolState();
  if (view === "cards") resetCardToolState();
  if (view === "cardReview") {
    cardReviewIndex.value = 0;
    cardAnswerVisible.value = false;
    reviewSessionDone.value = 0;
  }
  if (view === "knowledgeGraph") resetKnowledgeGraphToolState();
}

async function openWorkspace(options = {}) {
  const { push = true } = options;
  activeView.value = "workspace";
  noteMode.value = "preview";
  selectedId.value = null;
  selectedNote.value = null;
  noteInsights.value = null;
  saveState.value = "idle";
  if (push) writeRoute(null, false);
  void loadHomeIntelligence();
  void loadKnowledgeCards();
}

async function openToolView(view, options = {}) {
  resetToolViewState(view, options);
  activeView.value = view;
  noteMode.value = "preview";
  selectedId.value = null;
  selectedNote.value = null;
  noteInsights.value = null;
  saveState.value = "idle";
  writeRoute(null, false);
  if (["tasks", "templates", "import", "recommend", "qualityHub", "writingStudio", "calendar", "cards", "cardReview", "knowledgeGraph", "researchStudio"].includes(view)) {
    void loadHomeIntelligence();
  }
  if (view === "cards") void loadKnowledgeCards();
  if (view === "cardReview") void startCardReviewSession();
  if (view === "knowledgeGraph") void loadWorkspaceGraph();
  if (view === "researchStudio") {
    void loadResearchHistory(false);
    void loadHomeIntelligence();
  }
  if (view === "recommend") {
    void loadRecommendHistory(false);
  }
}

function openAssistantView() {
  activeView.value = "assistant";
  noteMode.value = "preview";
  selectedId.value = null;
  selectedNote.value = null;
  noteInsights.value = null;
  saveState.value = "idle";
  aiOpen.value = false;
  writeRoute(null, false);
  syncAIThreadMessages();
  focusAssistantInput();
}

function focusAssistantInput() {
  nextTick(() => {
    assistantInputRef.value?.focus?.();
  });
}

function startNewAssistantThread() {
  assistantThreadKey.value = `assistant:${Date.now()}`;
  aiInput.value = "";
  aiMessages.value = [];
  assistantAttachments.value = [];
  assistantAttachmentMenuOpen.value = false;
  assistantNotePickerOpen.value = false;
  aiHistoryOpen.value = false;
  focusAssistantInput();
}

function selectAssistantHistory(key) {
  assistantThreadKey.value = String(key || "assistant:main");
  aiMessages.value = readAIThreadMessages(assistantThreadKey.value);
  assistantAttachments.value = [];
  assistantAttachmentMenuOpen.value = false;
  assistantNotePickerOpen.value = false;
  aiHistoryOpen.value = false;
  scrollAIToBottom();
  focusAssistantInput();
}

function deleteAssistantHistory(key) {
  const target = String(key || "");
  if (!target) return;
  aiThreads.value.delete(target);
  saveAIThreads();
  if (target === assistantThreadKey.value) {
    assistantThreadKey.value = `assistant:${Date.now()}`;
    aiMessages.value = [];
    aiInput.value = "";
    assistantAttachments.value = [];
  }
  focusAssistantInput();
}

function fillAssistantPrompt(text) {
  aiInput.value = text;
  focusAssistantInput();
}

function setAssistantScope(scope) {
  assistantScope.value = scope === "workspace" ? "workspace" : "library";
  focusAssistantInput();
}

function toggleAssistantAttachmentMenu() {
  assistantAttachmentMenuOpen.value = !assistantAttachmentMenuOpen.value;
  if (!assistantAttachmentMenuOpen.value) assistantNotePickerOpen.value = false;
}

function openAssistantNotePicker() {
  assistantAttachmentMenuOpen.value = false;
  assistantNotePickerOpen.value = true;
  assistantNoteSearch.value = "";
}

function openAssistantFilePicker() {
  assistantAttachmentMenuOpen.value = false;
  assistantNotePickerOpen.value = false;
  assistantAttachmentFileInputRef.value?.click?.();
}

async function addAssistantNoteAttachment(note) {
  const id = Number(note?.id || 0);
  if (!id) return;
  if ((assistantAttachments.value || []).some((item) => item.type === "note" && Number(item.id) === id)) return;
  assistantAttachmentReading.value = true;
  try {
    const full = await api(`/api/notes/${id}`);
    assistantAttachments.value = [
      ...assistantAttachments.value,
      {
        type: "note",
        id,
        name: full.title || note.title || `笔记 #${id}`,
        content: String(full.markdown || "")
      }
    ];
    assistantNotePickerOpen.value = false;
  } finally {
    assistantAttachmentReading.value = false;
    focusAssistantInput();
  }
}

async function onAssistantFileChange(e) {
  const files = Array.from(e.target?.files || []);
  if (!files.length) return;
  assistantAttachmentReading.value = true;
  try {
    const next = [];
    for (const file of files) {
      const text = await file.text();
      next.push({
        type: "file",
        id: `${file.name}-${file.size}-${file.lastModified}`,
        name: file.name,
        content: text
      });
    }
    assistantAttachments.value = [...assistantAttachments.value, ...next];
  } finally {
    assistantAttachmentReading.value = false;
    if (e.target) e.target.value = "";
    focusAssistantInput();
  }
}

function removeAssistantAttachment(index) {
  assistantAttachments.value = (assistantAttachments.value || []).filter((_, i) => i !== index);
}

function assistantContextText() {
  const items = assistantAttachments.value || [];
  if (!items.length) return "";
  return items.map((item, index) => {
    const label = item.type === "note" ? "已选笔记" : "本地文件";
    const content = String(item.content || "").trim().slice(0, 5000);
    return `【${label} ${index + 1}：${item.name}】\n${content}`;
  }).join("\n\n---\n\n");
}

function assistantWorkspaceContextText() {
  const today = localDateKey(new Date());
  const openTasks = (planTasks.value || [])
    .filter((task) => !task.done)
    .sort((a, b) => {
      const aKey = `${a.due || "9999-12-31"} ${a.start_time || "99:99"}`;
      const bKey = `${b.due || "9999-12-31"} ${b.start_time || "99:99"}`;
      return aKey.localeCompare(bKey);
    })
    .slice(0, 80);
  const lines = openTasks.map((task) => {
    const due = task.due || "未设置日期";
    const time = formatTaskTime(task);
    const desc = String(task.description || "").trim();
    return `- ${task.title}；日期：${due}；时间：${time}；优先级：${priorityLabel(task.priority)}${desc ? `；备注：${desc}` : ""}`;
  });
  return [
    "【助手工作台：计划任务】",
    `今天日期：${today}`,
    `未完成计划数量：${openTasks.length}`,
    lines.length ? lines.join("\n") : "未完成计划：无"
  ].join("\n");
}

function assistantCombinedContextText() {
  return [assistantWorkspaceContextText(), assistantContextText()].filter(Boolean).join("\n\n---\n\n");
}

function assistantScopedContextText() {
  if (assistantScope.value === "library") {
    return assistantContextText();
  }
  return assistantCombinedContextText();
}

function fillTodayPlanPrompt() {
  const today = localDateKey(new Date());
  const tasks = (todayPlanTasks.value || []).map((task) => {
    const time = formatTaskTime(task);
    return `- ${task.title}${time ? `（${time}）` : ""}，优先级：${priorityLabel(task.priority)}`;
  });
  const planText = tasks.length ? tasks.join("\n") : "今天还没有写入计划。";
  const extra = String(aiInput.value || "").trim();
  fillAssistantPrompt(`请帮我整理今天的计划，给出执行顺序、注意事项和可以推迟的事项。\n\n今天日期：${today}\n今日计划：\n${planText}${extra ? `\n\n补充要求：${extra}` : ""}`);
}

function runAssistantAction(action) {
  if (action === "library") {
    setAssistantScope("library");
    return;
  }
  if (action === "today") {
    assistantScope.value = "workspace";
    fillTodayPlanPrompt();
    return;
  }
  if (action === "recommend") {
    const topic = String(aiInput.value || "").trim();
    if (topic) aiInput.value = "";
    openToolView("recommend", { recommendQuery: topic });
    return;
  }
  if (action === "weekly") {
    openToolView("writingStudio");
  }
}

function openGuide(key) {
  currentGuideKey.value = key || "workspace";
  guideModalOpen.value = true;
}

function closeGuide() {
  guideModalOpen.value = false;
}

function refTitle(ref) {
  return ref?.title || `Note #${ref?.id || ""}`;
}

function scorePercent(score) {
  const n = Number(score || 0);
  if (!Number.isFinite(n)) return "0%";
  return `${Math.max(1, Math.min(99, Math.round(n * 100)))}%`;
}

function chartTotal(items) {
  return (items || []).reduce((sum, item) => sum + Number(item.value || 0), 0);
}

function pieDashArray(item, items) {
  const total = chartTotal(items);
  if (!total) return "0 100";
  return `${Math.max(0, (Number(item.value || 0) / total) * 100)} 100`;
}

function pieDashOffset(index, items) {
  const total = chartTotal(items);
  if (!total) return 25;
  const prev = (items || []).slice(0, index).reduce((sum, item) => sum + Number(item.value || 0), 0);
  return 25 - (prev / total) * 100;
}

function chartMax(items) {
  return Math.max(1, ...(items || []).map((item) => Number(item.value || 0)));
}

function lineChartPoints(items) {
  const list = items || [];
  if (!list.length) return "";
  const max = chartMax(list);
  const step = list.length > 1 ? 280 / (list.length - 1) : 280;
  return list.map((item, index) => {
    const x = 10 + index * step;
    const y = 110 - (Number(item.value || 0) / max) * 90;
    return `${x},${y}`;
  }).join(" ");
}

function compareGraphEdges(a, b) {
  const typeRank = { link: 3, tag: 2, similar: 1 };
  const rankDiff = (typeRank[b.type] || 0) - (typeRank[a.type] || 0);
  if (rankDiff !== 0) return rankDiff;
  const weightDiff = Number(b.weight || 0) - Number(a.weight || 0);
  if (Math.abs(weightDiff) > 0.001) return weightDiff;
  if (Number(a.source) !== Number(b.source)) return Number(a.source) - Number(b.source);
  return Number(a.target) - Number(b.target);
}

function pickReadableGraphEdges(edges, limit, nodeCap) {
  const picked = [];
  const counts = new Map();
  for (const edge of [...(edges || [])].sort(compareGraphEdges)) {
    const source = Number(edge.source);
    const target = Number(edge.target);
    const sourceCount = counts.get(source) || 0;
    const targetCount = counts.get(target) || 0;
    if (sourceCount >= nodeCap || targetCount >= nodeCap) continue;
    picked.push(edge);
    counts.set(source, sourceCount + 1);
    counts.set(target, targetCount + 1);
    if (picked.length >= limit) break;
  }
  return picked;
}

function graphNodeMatchesQuery(node, query) {
  const q = String(query || "").trim().toLowerCase();
  if (!q) return true;
  return [node.title, node.path].join(" ").toLowerCase().includes(q);
}

function graphSearchEdgeDirection(edge, matched) {
  const sourceMatched = matched.has(Number(edge.source));
  const targetMatched = matched.has(Number(edge.target));
  if (sourceMatched) return "search-outgoing";
  if (targetMatched) return "search-incoming";
  return "";
}

function layoutGraphNodes(nodes, edges = [], activeTag = "", focusID = 0) {
  const list = (nodes || []).map((node) => ({ ...node }));
  if (!list.length) return [];
  const width = 1180;
  const height = 720;
  const marginX = 72;
  const marginY = 70;
  const cx = width / 2;
  const cy = height / 2;
  const degree = new Map(list.map((node) => [Number(node.id), 0]));
  for (const edge of edges || []) {
    const score = graphEdgeStrength(edge) * 6;
    degree.set(Number(edge.source), (degree.get(Number(edge.source)) || 0) + score);
    degree.set(Number(edge.target), (degree.get(Number(edge.target)) || 0) + score);
  }

  const clusters = graphClusterCenters(list, width, height, activeTag);
  const sorted = [...list].sort((a, b) => {
    const diff = (degree.get(Number(b.id)) || 0) - (degree.get(Number(a.id)) || 0);
    if (Math.abs(diff) > 0.001) return diff;
    return String(a.title || "").localeCompare(String(b.title || ""), "zh-CN");
  });
  const featured = new Set(sorted.slice(0, list.length <= 24 ? list.length : 14).map((node) => Number(node.id)));

  const groups = new Map();
  for (const node of list) {
    const cluster = graphPrimaryTag(node, activeTag);
    if (!groups.has(cluster)) groups.set(cluster, []);
    groups.get(cluster).push(node);
  }

  for (const [cluster, group] of groups.entries()) {
    const center = clusters.get(cluster) || { x: cx, y: cy };
    group.sort((a, b) => (degree.get(Number(b.id)) || 0) - (degree.get(Number(a.id)) || 0));
    if (Number(focusID || 0)) {
      const focusIndex = group.findIndex((node) => Number(node.id) === Number(focusID));
      if (focusIndex > 0) {
        const focusNode = group.splice(focusIndex, 1)[0];
        group.unshift(focusNode);
      }
    }
    const columns = Math.max(1, Math.ceil(Math.sqrt(group.length * 1.08)));
    const cellX = Number(focusID || 0) ? 164 : 142;
    const cellY = Number(focusID || 0) ? 116 : 96;
    const rows = Math.ceil(group.length / columns);
    group.forEach((node, index) => {
      const row = Math.floor(index / columns);
      const col = index % columns;
      const jitter = seededUnit(Number(node.id), index) - 0.5;
      const isFocus = Number(node.id) === Number(focusID || 0);
      const offsetX = (col - (columns - 1) / 2) * cellX + jitter * 12;
      const offsetY = (row - (rows - 1) / 2) * cellY + (seededUnit(Number(node.id) + 13, index) - 0.5) * 10;
      node.cluster = cluster;
      node.degree = Math.round((degree.get(Number(node.id)) || 0) * 10) / 10;
      node.radius = isFocus ? 20 : Math.max(8, Math.min(16, 8 + Math.sqrt(node.degree + 1) * 1.4));
      node.x = center.x + offsetX;
      node.y = center.y + offsetY;
      node.color = graphClusterColor(cluster);
      node.showLabel = isFocus || list.length <= 20 || featured.has(Number(node.id)) || (Boolean(activeTag) && list.length <= 32);
      node.label = shortenGraphTitle(node.title || "未命名", node.showLabel ? 20 : 10);
      node.labelWidth = Math.max(76, Math.min(210, visualTextLength(node.label) * 7.2 + 26));
    });
  }

  for (let step = 0; step < 56; step += 1) {
    for (let i = 0; i < list.length; i += 1) {
      for (let j = i + 1; j < list.length; j += 1) {
        const a = list[i];
        const b = list[j];
        const labelPad = (a.showLabel ? Math.min(72, a.labelWidth * 0.35) : 0) + (b.showLabel ? Math.min(72, b.labelWidth * 0.35) : 0);
        const minDist = a.radius + b.radius + 24 + labelPad;
        let dx = b.x - a.x;
        let dy = b.y - a.y;
        let dist = Math.sqrt(dx * dx + dy * dy) || 1;
        if (dist >= minDist) continue;
        const push = (minDist - dist) / 2;
        dx /= dist;
        dy /= dist;
        a.x -= dx * push;
        a.y -= dy * push;
        b.x += dx * push;
        b.y += dy * push;
      }
    }
    for (const node of list) {
      node.x = Math.max(marginX, Math.min(width - marginX, node.x));
      node.y = Math.max(marginY, Math.min(height - marginY, node.y));
    }
  }

  const occupied = list.map((node) => ({
    x: node.x - node.radius - 10,
    y: node.y - node.radius - 10,
    width: node.radius * 2 + 20,
    height: node.radius * 2 + 20
  }));
  const withLabels = [...list].sort((a, b) => {
    const focusDiff = (Number(b.id) === Number(focusID || 0) ? 1 : 0) - (Number(a.id) === Number(focusID || 0) ? 1 : 0);
    if (focusDiff !== 0) return focusDiff;
    return Number(b.degree || 0) - Number(a.degree || 0);
  });
  for (const node of withLabels) {
    if (!node.showLabel) continue;
    const labelBox = placeGraphLabel(node, occupied, width, height, cx, cy);
    node.labelX = labelBox.x;
    node.labelY = labelBox.y;
    occupied.push(labelBox);
  }

  return list.map((node) => {
    return {
      ...node,
      x: Math.round(node.x),
      y: Math.round(node.y),
      labelX: Math.round(node.labelX || node.x + node.radius + 12),
      labelY: Math.round(node.labelY || node.y - 12)
    };
  });
}

function placeGraphLabel(node, occupied, width, height, cx, cy) {
  const w = node.labelWidth;
  const h = 26;
  const gap = node.radius + 14;
  const preferRight = node.x < cx;
  const preferBottom = node.y < cy;
  const candidates = [
    { x: preferRight ? node.x + gap : node.x - gap - w, y: node.y - h / 2 },
    { x: preferRight ? node.x - gap - w : node.x + gap, y: node.y - h / 2 },
    { x: node.x - w / 2, y: preferBottom ? node.y + gap : node.y - gap - h },
    { x: node.x - w / 2, y: preferBottom ? node.y - gap - h : node.y + gap },
    { x: node.x + gap * 0.72, y: node.y + gap * 0.56 },
    { x: node.x - gap * 0.72 - w, y: node.y + gap * 0.56 },
    { x: node.x + gap * 0.72, y: node.y - gap * 0.56 - h },
    { x: node.x - gap * 0.72 - w, y: node.y - gap * 0.56 - h }
  ].map((box) => ({
    x: Math.max(16, Math.min(width - w - 16, box.x)),
    y: Math.max(18, Math.min(height - h - 18, box.y)),
    width: w,
    height: h
  }));

  let best = candidates[0];
  let bestScore = Number.POSITIVE_INFINITY;
  for (const box of candidates) {
    const overlap = occupied.reduce((score, other) => score + graphBoxOverlapArea(box, other), 0);
    const drift = Math.abs(box.x + box.width / 2 - node.x) * 0.08 + Math.abs(box.y + box.height / 2 - node.y) * 0.05;
    const edgePenalty = box.x <= 18 || box.y <= 20 || box.x + box.width >= width - 18 || box.y + box.height >= height - 20 ? 200 : 0;
    const score = overlap * 12 + drift + edgePenalty;
    if (score < bestScore) {
      best = box;
      bestScore = score;
    }
  }
  return best;
}

function graphBoxOverlapArea(a, b) {
  const x = Math.max(0, Math.min(a.x + a.width, b.x + b.width) - Math.max(a.x, b.x));
  const y = Math.max(0, Math.min(a.y + a.height, b.y + b.height) - Math.max(a.y, b.y));
  return x * y;
}

function focusGraphNode(id) {
  graphFocusID.value = Number(id || 0);
}

function clearGraphFocus() {
  graphFocusID.value = 0;
}

function selectGraphTopic(tag) {
  graphTagFilter.value = tag || "";
  graphFocusID.value = 0;
}

function seededUnit(id, index) {
  const seed = Math.sin((Number(id) || index + 1) * 12.9898) * 43758.5453;
  return seed - Math.floor(seed);
}

function graphClusterCenters(nodes, width, height, activeTag) {
  const tags = Array.from(new Set(nodes.map((node) => graphPrimaryTag(node, activeTag)))).sort((a, b) => a.localeCompare(b, "zh-CN"));
  const map = new Map();
  if (tags.length <= 1) {
    map.set(tags[0] || "未分类", { x: width / 2, y: height / 2 });
    return map;
  }
  const cx = width / 2;
  const cy = height / 2;
  const rx = width * 0.32;
  const ry = height * 0.27;
  tags.forEach((tag, index) => {
    const angle = (Math.PI * 2 * index) / tags.length - Math.PI / 2;
    map.set(tag, {
      x: cx + Math.cos(angle) * rx,
      y: cy + Math.sin(angle) * ry
    });
  });
  return map;
}

function graphPrimaryTag(node, activeTag = "") {
  if (activeTag) return activeTag;
  return (node.tags || []).find((tag) => tag && tag !== "folder") || "未分类";
}

function graphClusterColor(cluster) {
  const palette = ["#70d6ff", "#9bdb7c", "#f7c66f", "#f28c8c", "#b69cff", "#5bd8bd", "#ffb86b", "#90a8ff"];
  let hash = 0;
  for (const ch of String(cluster || "")) hash = (hash * 31 + ch.charCodeAt(0)) % 997;
  return palette[hash % palette.length];
}

function graphEdgeStrength(edge) {
  const base = edge.type === "link" ? 1.35 : edge.type === "tag" ? 0.78 : 0.56;
  return base * Math.max(0.35, Number(edge.weight || 0.5));
}

function graphEdgeLength(edge) {
  if (edge.type === "link") return 128;
  if (edge.type === "tag") return 176;
  return 210;
}

function seededAngle(id, index) {
  const seed = Math.sin((Number(id) || index + 1) * 12.9898) * 43758.5453;
  return (seed - Math.floor(seed)) * Math.PI * 2;
}

function shortenGraphTitle(title, max) {
  const text = String(title || "未命名").trim();
  const chars = Array.from(text);
  if (chars.length <= max) return text;
  return `${chars.slice(0, max - 1).join("")}...`;
}

function visualTextLength(text) {
  return Array.from(String(text || "")).reduce((sum, ch) => sum + (ch.charCodeAt(0) > 127 ? 1.7 : 1), 0);
}

function noteStatusLabel(status) {
  return status === "completed" ? "已完成" : "未完成";
}

function cardStatusLabel(status) {
  if (status === "mastered") return "已掌握";
  if (status === "archived") return "已归档";
  return "复习中";
}

function formatDateTime(raw) {
  if (!raw) return "未安排";
  const d = new Date(raw);
  if (Number.isNaN(d.getTime())) return String(raw);
  return d.toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" });
}

function openCardForm(card = null) {
  editingCardID.value = Number(card?.id || 0);
  cardForm.value = {
    front: card?.front || "",
    back: card?.back || ""
  };
  cardFormOpen.value = true;
}

function closeCardForm() {
  cardFormOpen.value = false;
  editingCardID.value = 0;
  cardForm.value = { front: "", back: "" };
}

async function saveKnowledgeCard() {
  const front = String(cardForm.value.front || "").trim();
  const back = String(cardForm.value.back || "").trim();
  if (!front || !back || cardSaving.value) return;
  cardSaving.value = true;
  try {
    const id = Number(editingCardID.value || 0);
    const old = id ? (knowledgeCards.value || []).find((card) => Number(card.id) === id) : null;
    const payload = {
      front,
      back,
      tags: id ? (old?.tags || []) : [],
      status: old?.status || "active"
    };
    if (id) {
      await api(`/api/cards/${id}`, { method: "PUT", body: JSON.stringify(payload) });
    } else {
      await api("/api/cards", { method: "POST", body: JSON.stringify(payload) });
    }
    closeCardForm();
    await Promise.all([loadKnowledgeCards(), loadWorkspaceDashboard()]);
  } finally {
    cardSaving.value = false;
  }
}

async function archiveKnowledgeCard(card) {
  if (!card?.id) return;
  await api(`/api/cards/${card.id}`, {
    method: "PUT",
    body: JSON.stringify({
      front: card.front,
      back: card.back,
      tags: card.tags || [],
      status: card.status === "archived" ? "active" : "archived"
    })
  });
  await Promise.all([loadKnowledgeCards(), loadWorkspaceDashboard()]);
}

async function deleteKnowledgeCard(card) {
  if (!card?.id) return;
  if (!window.confirm("确定删除这张知识卡片吗？")) return;
  await api(`/api/cards/${card.id}`, { method: "DELETE" });
  await Promise.all([loadKnowledgeCards(), loadWorkspaceDashboard()]);
}

async function reviewCurrentCard(remembered) {
  const card = currentReviewCard.value;
  if (!card || cardReviewing.value) return;
  cardReviewing.value = true;
  try {
    await api(`/api/cards/${card.id}/review`, {
      method: "POST",
      body: JSON.stringify({ remembered })
    });
    cardAnswerVisible.value = false;
    if (activeView.value === "cardReview") {
      reviewSessionCards.value = (reviewSessionCards.value || []).filter((item) => Number(item.id) !== Number(card.id));
      reviewSessionDone.value += 1;
      cardReviewIndex.value = 0;
      void loadKnowledgeCards();
      return;
    }
    await Promise.all([loadKnowledgeCards(), loadWorkspaceDashboard()]);
  } finally {
    cardReviewing.value = false;
  }
}

async function startCardReviewSession() {
  await loadKnowledgeCards();
  reviewSessionCards.value = [...(dueKnowledgeCards.value || [])];
  reviewSessionDone.value = 0;
  cardReviewIndex.value = 0;
  cardAnswerVisible.value = false;
}

async function restartKnowledgeCardReview(card) {
  if (!card?.id) return;
  await api(`/api/cards/${card.id}/review`, {
    method: "POST",
    body: JSON.stringify({ remembered: false })
  });
  await Promise.all([loadKnowledgeCards(), loadWorkspaceDashboard()]);
}

async function runResearchSession() {
  const topic = String(researchTopic.value || "").trim();
  if (!topic || researchLoading.value) return;
  researchLoading.value = true;
  researchResult.value = null;
  researchHistoryOpen.value = false;
  try {
    const result = await api("/api/research/session", {
      method: "POST",
      body: JSON.stringify({ topic })
    });
    researchResult.value = result;
    await loadResearchHistory(false);
  } finally {
    researchLoading.value = false;
  }
}

async function loadResearchHistory(openLatest = false) {
  try {
    const items = await api("/api/research/sessions");
    researchHistory.value = Array.isArray(items)
      ? items.map(normalizeResearchHistoryItem).filter(Boolean).slice(0, 50)
      : [];
    if (openLatest && !researchResult.value && researchHistoryItems.value.length) {
      openResearchHistory(researchHistoryItems.value[0]);
    }
  } catch {
    researchHistory.value = [];
  }
}

function normalizeResearchHistoryItem(item) {
  const result = item?.result || item;
  const topic = String(result?.topic || item?.topic || "").trim();
  if (!topic || !result) return null;
  return {
    id: String(item?.id || `${Date.now()}-${Math.random().toString(16).slice(2)}`),
    topic,
    createdAt: researchCreatedAtValue(item),
    result: {
      ...result,
      id: Number(result?.id || item?.id || 0),
      created_at: result?.created_at || item?.created_at || "",
      topic
    }
  };
}

function researchCreatedAtValue(item) {
  const raw = item?.created_at || item?.createdAt || item?.result?.created_at || "";
  const time = raw ? new Date(raw).getTime() : Number(item?.createdAt || 0);
  return Number.isFinite(time) && time > 0 ? time : Date.now();
}

function openResearchHistory(item) {
  const normalized = normalizeResearchHistoryItem(item);
  if (!normalized) return;
  researchTopic.value = normalized.topic;
  researchResult.value = normalized.result;
  researchHistoryOpen.value = false;
}

function onResearchTopicInput() {
  if (!researchResult.value) return;
  const current = String(researchTopic.value || "").trim();
  const shown = String(researchResult.value.topic || "").trim();
  if (current !== shown) {
    researchResult.value = null;
  }
}

async function deleteResearchHistory(item) {
  const id = Number(item?.id || 0);
  if (!id) return;
  await api(`/api/research/sessions/${id}`, { method: "DELETE" });
  researchHistory.value = (researchHistory.value || []).filter((entry) => Number(entry.id) !== id);
  if (researchResult.value?.topic === item?.topic) {
    researchResult.value = researchHistoryItems.value[0]?.result || null;
    researchTopic.value = researchResult.value?.topic || "";
  }
}

async function loadRecommendHistory(openLatest = false) {
  try {
    const items = await api("/api/recommend/sessions");
    recommendHistory.value = Array.isArray(items)
      ? items.map(normalizeRecommendHistoryItem).filter(Boolean).slice(0, 50)
      : [];
    if (openLatest && !recommendResult.value && recommendHistoryItems.value.length) {
      openRecommendHistory(recommendHistoryItems.value[0]);
    }
  } catch {
    recommendHistory.value = [];
  }
}

function normalizeRecommendHistoryItem(item) {
  const result = item?.result || item;
  const topic = String(result?.topic || item?.topic || "").trim();
  if (!topic || !result) return null;
  return {
    id: String(item?.id || `${Date.now()}-${Math.random().toString(16).slice(2)}`),
    topic,
    createdAt: recommendCreatedAtValue(item),
    result: {
      ...result,
      id: Number(result?.id || item?.id || 0),
      created_at: result?.created_at || item?.created_at || "",
      topic
    }
  };
}

function recommendCreatedAtValue(item) {
  const raw = item?.created_at || item?.createdAt || item?.result?.created_at || "";
  const time = raw ? new Date(raw).getTime() : Number(item?.createdAt || 0);
  return Number.isFinite(time) && time > 0 ? time : Date.now();
}

function openRecommendHistory(item) {
  const normalized = normalizeRecommendHistoryItem(item);
  if (!normalized) return;
  recommendQuery.value = normalized.topic;
  recommendResult.value = normalized.result;
  recommendHistoryOpen.value = false;
}

function onRecommendTopicInput() {
  if (!recommendResult.value) return;
  const current = String(recommendQuery.value || "").trim();
  const shown = String(recommendResult.value.topic || "").trim();
  if (current !== shown) {
    recommendResult.value = null;
  }
}

async function deleteRecommendHistory(item) {
  const id = Number(item?.id || 0);
  if (!id) return;
  await api(`/api/recommend/sessions/${id}`, { method: "DELETE" });
  recommendHistory.value = (recommendHistory.value || []).filter((entry) => Number(entry.id) !== id);
  if (Number(recommendResult.value?.id || 0) === id || recommendResult.value?.topic === item?.topic) {
    recommendResult.value = recommendHistoryItems.value[0]?.result || null;
    recommendQuery.value = recommendResult.value?.topic || "";
  }
}

async function createCardFromResearchQuestion(question) {
  const front = String(question || "").trim();
  if (!front || researchCardCreating.value) return;
  researchCardCreating.value = front;
  try {
    await api("/api/cards", {
      method: "POST",
      body: JSON.stringify({
        front,
        back: `围绕“${researchResult.value?.topic || researchTopic.value}”整理答案。`,
        tags: ["research"],
        status: "active"
      })
    });
    await Promise.all([loadKnowledgeCards(), loadWorkspaceDashboard()]);
  } finally {
    researchCardCreating.value = "";
  }
}

async function createTemplateNote(key) {
  const tpl = (workspaceTemplates.value || []).find((item) => item.key === key);
  const title = window.prompt("给模板笔记起个标题", tpl?.name || "New Note");
  if (title === null) return;
  let note;
  const isBaseTemplate = (templates.value || []).some((item) => item.key === key);
  const isCustomized = (templatePrefs.value.custom || []).some((item) => item.key === key);
  if (isBaseTemplate && !isCustomized) {
    note = await api(`/api/templates/${key}/notes`, {
      method: "POST",
      body: JSON.stringify({ title: String(title || "").trim() })
    });
  } else {
    note = await api("/api/notes", {
      method: "POST",
      body: JSON.stringify({
        title: String(title || "").trim() || tpl?.name || "New Note",
        markdown: tpl?.markdown || "# New Note\n",
        tags: tpl?.tags || []
      })
    });
  }
  await Promise.all([loadNotes(), loadAllNotes(), loadHomeIntelligence()]);
  await openNote(note.id);
  noteMode.value = "edit";
}

function openImportModal() {
  importTitle.value = "";
  importText.value = "";
  importTags.value = [];
  importTagQuery.value = "";
  importTagMenuOpen.value = false;
  importFileName.value = "";
  importParentID.value = "";
  importModalOpen.value = true;
}

function closeImportModal() {
  importModalOpen.value = false;
}

async function submitImportNote() {
  const markdownText = String(importText.value || "").trim();
  if (!markdownText) return;
  const note = await api("/api/import", {
    method: "POST",
    body: JSON.stringify({
      title: importTitle.value,
      markdown: markdownText,
      parent_id: importParentID.value ? Number(importParentID.value) : null,
      tags: normalizeTags(importTags.value || [])
    })
  });
  closeImportModal();
  await Promise.all([loadNotes(), loadAllNotes(), loadHomeIntelligence(), loadTags()]);
  await openNote(note.id);
}

async function onImportFileChange(e) {
  const file = e.target?.files?.[0];
  if (!file) return;
  importReading.value = true;
  importFileName.value = file.name;
  if (!importTitle.value) {
    importTitle.value = file.name.replace(/\.[^.]+$/, "");
  }
  try {
    importText.value = await file.text();
  } catch (err) {
    importText.value = "";
    window.alert(`读取文件失败：${err?.message || err}`);
  } finally {
    importReading.value = false;
  }
}

function toggleImportTag(tag) {
  const clean = String(tag || "").trim();
  if (!clean) return;
  const current = normalizeTags(importTags.value || []);
  const exists = current.some((item) => item.toLowerCase() === clean.toLowerCase());
  importTags.value = exists
    ? current.filter((item) => item.toLowerCase() !== clean.toLowerCase())
    : normalizeTags([...current, clean]);
}

function removeImportTag(tag) {
  importTags.value = normalizeTags(importTags.value || []).filter((item) => item.toLowerCase() !== String(tag).toLowerCase());
}

function createImportTagFromQuery() {
  const clean = String(importTagQuery.value || "").trim();
  if (!clean) return;
  toggleImportTag(clean);
  mergeTagCatalog([clean]);
  importTagQuery.value = "";
}

async function createFolderForTarget(target) {
  const title = window.prompt("新建文件夹名称", "新文件夹");
  if (title === null) return;
  const clean = String(title || "").trim();
  if (!clean) return;
  const res = await api("/api/notes", {
    method: "POST",
    body: JSON.stringify({
      title: clean,
      markdown: `# ${clean}\n\n用于归类页面。`,
      parent_id: null,
      tags: ["folder"]
    })
  });
  const folder = res.note || res;
  await Promise.all([loadNotes(), loadAllNotes(), loadTags()]);
  folderOpen.value[folder.id] = true;
  if (target === "import") importParentID.value = String(folder.id);
  if (target === "weekly") weeklyReportParentID.value = String(folder.id);
  if (target === "quickMemo") quickMemoParentID.value = String(folder.id);
}

function toggleRecommendNote(note) {
  const id = Number(note?.id || note);
  if (!id) return;
  const current = recommendSelectedIDs.value || [];
  if (current.some((item) => Number(item) === id)) {
    recommendSelectedIDs.value = current.filter((item) => Number(item) !== id);
    return;
  }
  recommendSelectedIDs.value = [...current, id];
}

function clearRecommendNotes() {
  recommendSelectedIDs.value = [];
}

function toggleWeeklyNote(note) {
  const id = Number(note?.id || note);
  if (!id) return;
  const current = weeklySelectedIDs.value || [];
  if (current.some((item) => Number(item) === id)) {
    weeklySelectedIDs.value = current.filter((item) => Number(item) !== id);
    return;
  }
  weeklySelectedIDs.value = [...current, id];
}

function clearWeeklySources() {
  weeklySelectedIDs.value = [];
  weeklyLocalFiles.value = [];
}

function openWeeklyFilePicker() {
  weeklyFileInputRef.value?.click?.();
}

async function onWeeklyFileChange(e) {
  const files = Array.from(e.target?.files || []);
  if (!files.length) return;
  weeklyFileReading.value = true;
  try {
    const next = [];
    for (const file of files) {
      const text = await file.text();
      next.push({
        id: `${file.name}-${file.size}-${file.lastModified}`,
        name: file.name,
        content: text
      });
    }
    const existing = new Set((weeklyLocalFiles.value || []).map((item) => item.id));
    weeklyLocalFiles.value = [
      ...(weeklyLocalFiles.value || []),
      ...next.filter((item) => !existing.has(item.id))
    ];
  } finally {
    weeklyFileReading.value = false;
    if (e.target) e.target.value = "";
  }
}

function removeWeeklyLocalFile(index) {
  weeklyLocalFiles.value = (weeklyLocalFiles.value || []).filter((_, i) => i !== index);
}

function openFirstNote(note) {
  if (note?.id) openNote(note.id);
}

function plainNoteText(note) {
  return String(note?.markdown || "")
    .replace(/```[\s\S]*?```/g, " ")
    .replace(/[#>*_`~\-[\]()]/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

function defaultWeeklyReportTitle() {
  return `本周学习周报 ${new Date().toISOString().slice(0, 10)}`;
}

async function generateWeeklyReport() {
  weeklyReportLoading.value = true;
  weeklyReportResult.value = null;
  try {
    const result = await api("/api/writing/weekly-report", {
      method: "POST",
      body: JSON.stringify({
        title: String(weeklyReportTitle.value || "").trim() || defaultWeeklyReportTitle(),
        parent_id: weeklyReportParentID.value ? Number(weeklyReportParentID.value) : null,
        note_ids: (weeklySelectedIDs.value || []).map((id) => Number(id)).filter((id) => id > 0),
        file_sources: (weeklyLocalFiles.value || []).map((file) => ({
          name: file.name,
          content: String(file.content || "")
        }))
      })
    });
    weeklyReportResult.value = result;
    await Promise.all([loadNotes(), loadAllNotes(), loadHomeIntelligence(), loadTags()]);
  } catch (err) {
    weeklyReportResult.value = {
      error: err?.message || String(err)
    };
  } finally {
    weeklyReportLoading.value = false;
  }
}

async function runAIRecommendation() {
  const topic = String(recommendQuery.value || "").trim();
  if (!topic && recommendSelectedIDs.value.length === 0) return;
  recommendLoading.value = true;
  recommendResult.value = null;
  recommendHistoryOpen.value = false;
  try {
    recommendResult.value = await api("/api/recommend", {
      method: "POST",
      body: JSON.stringify({
        topic,
        note_ids: recommendSelectedIDs.value.map((id) => Number(id)).filter((id) => id > 0)
      })
    });
    await loadRecommendHistory(false);
  } catch (err) {
    recommendResult.value = {
      summary: `生成推荐失败：${err?.message || err}`,
      recommendations: [],
      references: [],
      resources: []
    };
  } finally {
    recommendLoading.value = false;
  }
}

function loadPlanTasks() {
  try {
    const raw = localStorage.getItem(PLAN_TASK_KEY);
    const parsed = raw ? JSON.parse(raw) : [];
    planTasks.value = mergeDefaultPlanTasks(Array.isArray(parsed) ? parsed : []);
  } catch {
    planTasks.value = defaultPlanTasks();
  }
  savePlanTasks();
}

function savePlanTasks() {
  localStorage.setItem(PLAN_TASK_KEY, JSON.stringify((planTasks.value || []).slice(0, 300)));
}

function parseLocalDate(value) {
  const raw = String(value || "").trim();
  const m = raw.match(/^(\d{4})-(\d{2})-(\d{2})$/);
  if (m) return new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]));
  const d = new Date(raw || Date.now());
  if (Number.isNaN(d.getTime())) return new Date();
  return d;
}

function localDateKey(dateLike) {
  const d = dateLike instanceof Date ? dateLike : parseLocalDate(dateLike);
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

function addDays(dateLike, amount) {
  const d = parseLocalDate(dateLike);
  d.setDate(d.getDate() + amount);
  return d;
}

function startOfWeek(dateLike) {
  const d = parseLocalDate(dateLike);
  d.setDate(d.getDate() - d.getDay());
  return d;
}

function formatCalendarMonth(dateLike) {
  const d = parseLocalDate(dateLike);
  return d.toLocaleDateString("zh-CN", { year: "numeric", month: "long" });
}

function formatCalendarDateLong(dateLike) {
  const d = parseLocalDate(dateLike);
  return d.toLocaleDateString("zh-CN", { year: "numeric", month: "long", day: "numeric", weekday: "long" });
}

function buildCalendarMonth(dateLike) {
  const selected = parseLocalDate(dateLike);
  const first = new Date(selected.getFullYear(), selected.getMonth(), 1);
  const start = addDays(first, -first.getDay());
  const today = localDateKey(new Date());
  const picked = localDateKey(selected);
  return Array.from({ length: 42 }, (_, i) => {
    const d = addDays(start, i);
    const key = localDateKey(d);
    return {
      key,
      day: d.getDate(),
      currentMonth: d.getMonth() === selected.getMonth(),
      today: key === today,
      selected: key === picked,
      tasks: tasksForDate(key)
    };
  });
}

function buildCalendarWeek(dateLike) {
  const start = startOfWeek(dateLike);
  const today = localDateKey(new Date());
  const picked = localDateKey(dateLike);
  const names = ["周日", "周一", "周二", "周三", "周四", "周五", "周六"];
  return Array.from({ length: 7 }, (_, i) => {
    const d = addDays(start, i);
    const key = localDateKey(d);
    return {
      key,
      label: names[d.getDay()],
      day: d.getDate(),
      today: key === today,
      selected: key === picked,
      tasks: tasksForDate(key)
    };
  });
}

function tasksForDate(dateKey) {
  return (planTasks.value || []).filter((task) => String(task.due || "") === String(dateKey || ""));
}

function planTaskHour(task) {
  const time = normalizePlanTime(task?.start_time || "");
  if (!time) return 9;
  return Number(time.slice(0, 2));
}

function tasksForDateHour(dateKey, hour) {
  return tasksForDate(dateKey).filter((task) => planTaskHour(task) === Number(hour));
}

function formatTaskTime(task) {
  const { start, end } = normalizedPlanTimeRange(task);
  if (start && end) return `${start} - ${end}`;
  if (start) return start;
  return "未设置时间";
}

function defaultPlanTasks() {
  const today = new Date();
  const due = (offset) => {
    const d = new Date(today);
    d.setDate(today.getDate() + offset);
    return d.toISOString().slice(0, 10);
  };
  return [
    {
      id: 26051201,
      title: "检查文件导入的目标文件夹显示",
      due: due(0),
      start_time: "09:00",
      end_time: "10:00",
      description: "确认导入页面可选择已有文件夹并支持新建文件夹。",
      priority: "high",
      done: false,
      createdAt: today.toISOString()
    },
    {
      id: 26051202,
      title: "为推荐与回顾选择 3 篇参考笔记",
      due: due(0),
      start_time: "14:00",
      end_time: "15:00",
      description: "",
      priority: "medium",
      done: false,
      createdAt: today.toISOString()
    },
    {
      id: 26051203,
      title: "整理任务中心说明书和演示路径",
      due: due(1),
      start_time: "10:00",
      end_time: "11:00",
      description: "",
      priority: "medium",
      done: false,
      createdAt: today.toISOString()
    },
    {
      id: 26051204,
      title: "复盘 AI 与检索目录下的 RAG 笔记",
      due: due(2),
      start_time: "16:00",
      end_time: "17:00",
      description: "",
      priority: "low",
      done: false,
      createdAt: today.toISOString()
    },
    {
      id: 26051205,
      title: "完成模板库标签下拉多选验收",
      due: due(-1),
      start_time: "11:00",
      end_time: "12:00",
      description: "",
      priority: "low",
      done: true,
      createdAt: today.toISOString()
    }
  ];
}

function mergeDefaultPlanTasks(tasks) {
  const existing = Array.isArray(tasks) ? tasks : [];
  const seen = new Set(existing.map((task) => Number(task.id)));
  const missing = defaultPlanTasks().filter((task) => !seen.has(Number(task.id)));
  return [...existing, ...missing].map(normalizePlanTask);
}

function normalizePlanTask(task) {
  const due = String(task?.due || "");
  const { start, end } = normalizedPlanTimeRange({ ...task, due });
  return {
    ...task,
    id: Number(task?.id || Date.now()),
    title: String(task?.title || "未命名 plan"),
    due,
    start_time: start,
    end_time: end,
    description: String(task?.description || ""),
    priority: ["high", "medium", "low"].includes(task?.priority) ? task.priority : "medium",
    done: !!task?.done,
    createdAt: task?.createdAt || new Date().toISOString()
  };
}

function normalizePlanTime(raw) {
  const text = String(raw || "").trim();
  const m = text.match(/^(\d{1,2})(?::(\d{2}))?$/);
  if (!m) return "";
  const h = Math.max(0, Math.min(23, Number(m[1])));
  const minute = Math.max(0, Math.min(59, Number(m[2] || 0)));
  return `${String(h).padStart(2, "0")}:${String(minute).padStart(2, "0")}`;
}

function planTimeMinutes(raw) {
  const time = normalizePlanTime(raw);
  if (!time) return -1;
  const [hour, minute] = time.split(":").map(Number);
  return hour * 60 + minute;
}

function defaultPlanEndTime(startTime) {
  const start = normalizePlanTime(startTime);
  if (!start) return "";
  const total = Math.min(23 * 60 + 59, planTimeMinutes(start) + 60);
  return `${String(Math.floor(total / 60)).padStart(2, "0")}:${String(total % 60).padStart(2, "0")}`;
}

function normalizedPlanTimeRange(task) {
  const due = String(task?.due || "");
  const start = normalizePlanTime(task?.start_time || task?.startTime || "") || (due ? "09:00" : "");
  const rawEnd = normalizePlanTime(task?.end_time || task?.endTime || "");
  const end = rawEnd && planTimeMinutes(rawEnd) > planTimeMinutes(start) ? rawEnd : defaultPlanEndTime(start);
  return { start, end };
}

function addPlanTask(dueOverride = "") {
  const text = String(planTaskTitle.value || "").trim();
  if (!text) return;
  const explicitDue = typeof dueOverride === "string" ? dueOverride : "";
  const due = explicitDue || planTaskDate.value || "";
  const start = normalizePlanTime(planTaskStartTime.value || "09:00");
  const end = normalizePlanTime(planTaskEndTime.value || "") || defaultPlanEndTime(start);
  planTasks.value = [{
    id: Date.now(),
    title: text,
    due,
    start_time: start,
    end_time: planTimeMinutes(end) > planTimeMinutes(start) ? end : defaultPlanEndTime(start),
    description: "",
    priority: planTaskPriority.value || "medium",
    done: false,
    createdAt: new Date().toISOString()
  }, ...(planTasks.value || [])];
  planTaskTitle.value = "";
  savePlanTasks();
}

function togglePlanTask(task) {
  planTasks.value = (planTasks.value || []).map((item) =>
    item.id === task.id ? { ...item, done: !item.done } : item
  );
  savePlanTasks();
}

function deletePlanTask(task) {
  planTasks.value = (planTasks.value || []).filter((item) => item.id !== task.id);
  if (Number(calendarSelectedTaskID.value || 0) === Number(task.id)) {
    calendarSelectedTaskID.value = null;
  }
  savePlanTasks();
}

function selectCalendarDay(dateKey) {
  calendarSelectedDate.value = String(dateKey || localDateKey(new Date()));
  planTaskDate.value = calendarSelectedDate.value;
  calendarSelectedTaskID.value = null;
}

function createPlanOnDate(dateKey = calendarSelectedDate.value) {
  const title = window.prompt("创建 plan", "");
  if (title === null) return;
  const text = String(title || "").trim();
  if (!text) return;
  const start = normalizePlanTime(planTaskStartTime.value || "09:00");
  const end = normalizePlanTime(planTaskEndTime.value || "") || defaultPlanEndTime(start);
  const task = {
    id: Date.now(),
    title: text,
    due: dateKey || "",
    start_time: start,
    end_time: planTimeMinutes(end) > planTimeMinutes(start) ? end : defaultPlanEndTime(start),
    description: "",
    priority: planTaskPriority.value || "medium",
    done: false,
    createdAt: new Date().toISOString()
  };
  planTasks.value = [task, ...(planTasks.value || [])];
  calendarSelectedTaskID.value = task.id;
  savePlanTasks();
}

function createPlanAt(dateKey, hour = 9) {
  const start = `${String(hour).padStart(2, "0")}:00`;
  const end = defaultPlanEndTime(start);
  const task = {
    id: Date.now(),
    title: "新建 plan",
    due: dateKey || calendarSelectedDate.value,
    start_time: start,
    end_time: end,
    description: "",
    priority: "medium",
    done: false,
    createdAt: new Date().toISOString()
  };
  planTasks.value = [task, ...(planTasks.value || [])];
  calendarSelectedDate.value = task.due;
  calendarSelectedTaskID.value = task.id;
  savePlanTasks();
}

function updatePlanTaskField(task, field, value) {
  if (!task?.id) return;
  const nextValue = field === "start_time" || field === "end_time" ? normalizePlanTime(value) : value;
  planTasks.value = (planTasks.value || []).map((item) =>
    Number(item.id) === Number(task.id) ? normalizePlanTask({ ...item, [field]: nextValue }) : item
  );
  if (field === "due") calendarSelectedDate.value = String(nextValue || calendarSelectedDate.value);
  savePlanTasks();
}

function selectCalendarTask(task) {
  if (!task?.id) return;
  calendarSelectedTaskID.value = Number(task.id);
  if (task.due) calendarSelectedDate.value = String(task.due);
}

function openCalendarContextMenu(e, dateKey, hour) {
  calendarContextDate.value = String(dateKey || calendarSelectedDate.value);
  calendarContextHour.value = Number(hour || 9);
  calendarContextMenuX.value = Math.min(e.clientX, window.innerWidth - 230);
  calendarContextMenuY.value = Math.min(e.clientY, window.innerHeight - 120);
  calendarContextMenuOpen.value = true;
}

function closeCalendarContextMenu() {
  calendarContextMenuOpen.value = false;
}

function createPlanFromContext() {
  createPlanAt(calendarContextDate.value, calendarContextHour.value);
  closeCalendarContextMenu();
}

function openCalendarCommand() {
  calendarCommandQuery.value = "";
  calendarCommandCursor.value = 0;
  calendarCommandOpen.value = true;
  nextTick(() => {
    calendarCommandInputRef.value?.focus?.();
  });
}

function closeCalendarCommand() {
  calendarCommandOpen.value = false;
  calendarCommandQuery.value = "";
  calendarCommandCursor.value = 0;
}

function openCalendarPlanSearch() {
  calendarCommandOpen.value = false;
  calendarCommandQuery.value = "";
  calendarPlanSearchQuery.value = "";
  calendarPlanSearchCursor.value = 0;
  calendarPlanSearchOpen.value = true;
  nextTick(() => {
    calendarPlanSearchInputRef.value?.focus?.();
  });
}

function closeCalendarPlanSearch() {
  calendarPlanSearchOpen.value = false;
  calendarPlanSearchQuery.value = "";
  calendarPlanSearchCursor.value = 0;
}

function selectCalendarSearchResult(task) {
  if (!task?.id) return;
  selectCalendarTask(task);
  closeCalendarPlanSearch();
}

function moveCalendarCommandCursor(delta) {
  const list = calendarCommands.value || [];
  if (!list.length) {
    calendarCommandCursor.value = 0;
    return;
  }
  const current = Number(calendarCommandCursor.value || 0);
  calendarCommandCursor.value = (current + delta + list.length) % list.length;
}

function moveCalendarPlanSearchCursor(delta) {
  const list = calendarPlanSearchResults.value || [];
  if (!list.length) {
    calendarPlanSearchCursor.value = 0;
    return;
  }
  const current = Number(calendarPlanSearchCursor.value || 0);
  calendarPlanSearchCursor.value = (current + delta + list.length) % list.length;
}

function commandShortcutKey(command) {
  const hint = String(command?.hint || "").trim().toLowerCase();
  if (!hint) return "";
  const parts = hint.split(/\s+/);
  return parts[parts.length - 1] || "";
}

function keyboardEventKey(e) {
  const key = String(e.key || "").toLowerCase();
  if (key.length === 1) return key;
  const code = String(e.code || "").toLowerCase();
  if (code.startsWith("key")) return code.slice(3);
  if (code.startsWith("digit")) return code.slice(5);
  if (code === "period") return ".";
  if (code === "slash") return "/";
  return key;
}

function findCalendarCommandByShortcut(e) {
  if (calendarCommandQuery.value.trim()) return null;
  const key = keyboardEventKey(e);
  if (!key) return null;
  return (calendarCommands.value || []).find((command) => {
    const shortcut = commandShortcutKey(command);
    if (!shortcut || shortcut !== key) return false;
    const wantsAlt = String(command.hint || "").toLowerCase().includes("alt") || String(command.hint || "").toLowerCase().includes("option");
    if (wantsAlt) return e.altKey;
    return !e.altKey && !e.ctrlKey && !e.metaKey;
  }) || null;
}

function onCalendarCommandKeydown(e) {
  if (e.key === "ArrowDown") {
    e.preventDefault();
    moveCalendarCommandCursor(1);
    return;
  }
  if (e.key === "ArrowUp") {
    e.preventDefault();
    moveCalendarCommandCursor(-1);
    return;
  }
  if (e.key === "Home") {
    e.preventDefault();
    calendarCommandCursor.value = 0;
    return;
  }
  if (e.key === "End") {
    e.preventDefault();
    calendarCommandCursor.value = Math.max(0, (calendarCommands.value || []).length - 1);
    return;
  }
  if (e.key === "Enter") {
    e.preventDefault();
    const list = calendarCommands.value || [];
    runCalendarCommand(list[calendarCommandCursor.value] || list[0]);
    return;
  }
  if (e.key === "Escape") {
    e.preventDefault();
    closeCalendarCommand();
    return;
  }
  const shortcut = findCalendarCommandByShortcut(e);
  if (shortcut) {
    e.preventDefault();
    runCalendarCommand(shortcut);
  }
}

function onCalendarPlanSearchKeydown(e) {
  if (e.key === "ArrowDown") {
    e.preventDefault();
    moveCalendarPlanSearchCursor(1);
    return;
  }
  if (e.key === "ArrowUp") {
    e.preventDefault();
    moveCalendarPlanSearchCursor(-1);
    return;
  }
  if (e.key === "Home") {
    e.preventDefault();
    calendarPlanSearchCursor.value = 0;
    return;
  }
  if (e.key === "End") {
    e.preventDefault();
    calendarPlanSearchCursor.value = Math.max(0, (calendarPlanSearchResults.value || []).length - 1);
    return;
  }
  if (e.key === "Enter") {
    e.preventDefault();
    const list = calendarPlanSearchResults.value || [];
    selectCalendarSearchResult(list[calendarPlanSearchCursor.value] || list[0]);
    return;
  }
  if (e.key === "Escape") {
    e.preventDefault();
    closeCalendarPlanSearch();
  }
}

function runCalendarCommand(command) {
  const key = typeof command === "string" ? command : command?.key;
  if (!key) return;
  closeCalendarCommand();
  if (key === "create") {
    createPlanOnDate(calendarSelectedDate.value);
    return;
  }
  if (key === "goto-date") {
    const next = window.prompt("跳转到日期（YYYY-MM-DD）", calendarSelectedDate.value);
    if (next === null) return;
    const clean = localDateKey(parseLocalDate(next));
    selectCalendarDay(clean);
    return;
  }
  if (key === "today" || key === "align-today") {
    selectCalendarDay(localDateKey(new Date()));
    return;
  }
  if (key === "next-week") {
    selectCalendarDay(localDateKey(addDays(calendarSelectedDate.value, 7)));
    return;
  }
  if (key === "prev-week") {
    selectCalendarDay(localDateKey(addDays(calendarSelectedDate.value, -7)));
    return;
  }
  if (key === "search") {
    openCalendarPlanSearch();
  }
}

function priorityLabel(priority) {
  if (priority === "high") return "High";
  if (priority === "low") return "Low";
  return "Medium";
}

function loadTemplatePrefs() {
  try {
    const raw = localStorage.getItem(TEMPLATE_PREF_KEY);
    const parsed = raw ? JSON.parse(raw) : {};
    templatePrefs.value = {
      custom: Array.isArray(parsed.custom) ? parsed.custom : [],
      deleted: Array.isArray(parsed.deleted) ? parsed.deleted : []
    };
  } catch {
    templatePrefs.value = { custom: [], deleted: [] };
  }
}

function saveTemplatePrefs() {
  localStorage.setItem(TEMPLATE_PREF_KEY, JSON.stringify(templatePrefs.value));
}

function openTemplateModal(tpl = null) {
  editingTemplateKey.value = tpl?.key || "";
  templateForm.value = {
    name: tpl?.name || "",
    tags: normalizeTags(tpl?.tags || []),
    markdown: tpl?.markdown || "# 新模板\n\n"
  };
  templateTagQuery.value = "";
  templateTagMenuOpen.value = false;
  templateModalOpen.value = true;
}

function closeTemplateModal() {
  templateModalOpen.value = false;
}

function saveTemplate() {
  const name = String(templateForm.value.name || "").trim();
  const markdown = String(templateForm.value.markdown || "").trim();
  if (!name || !markdown) return;
  const key = editingTemplateKey.value || `custom-${Date.now()}`;
  const tpl = {
    key,
    name,
    tags: normalizeTags(templateForm.value.tags || []),
    markdown
  };
  const custom = (templatePrefs.value.custom || []).filter((item) => item.key !== key);
  templatePrefs.value = {
    custom: [tpl, ...custom],
    deleted: (templatePrefs.value.deleted || []).filter((item) => item !== key)
  };
  saveTemplatePrefs();
  closeTemplateModal();
}

function deleteTemplate(tpl) {
  if (!tpl?.key) return;
  const custom = (templatePrefs.value.custom || []).filter((item) => item.key !== tpl.key);
  const deleted = new Set(templatePrefs.value.deleted || []);
  deleted.add(tpl.key);
  templatePrefs.value = { custom, deleted: Array.from(deleted) };
  saveTemplatePrefs();
}

function toggleTemplateTag(tag) {
  const clean = String(tag || "").trim();
  if (!clean) return;
  const current = normalizeTags(templateForm.value.tags || []);
  const exists = current.some((item) => item.toLowerCase() === clean.toLowerCase());
  templateForm.value.tags = exists
    ? current.filter((item) => item.toLowerCase() !== clean.toLowerCase())
    : normalizeTags([...current, clean]);
}

function removeTemplateTag(tag) {
  templateForm.value.tags = normalizeTags(templateForm.value.tags || []).filter((item) => item.toLowerCase() !== String(tag).toLowerCase());
}

function createTemplateTagFromQuery() {
  const clean = String(templateTagQuery.value || "").trim();
  if (!clean) return;
  toggleTemplateTag(clean);
  mergeTagCatalog([clean]);
  templateTagQuery.value = "";
}

function applySuggestedTag(tag) {
  const clean = String(tag || "").trim();
  if (!clean) return;
  if (!selectedTags.value.some((item) => String(item).toLowerCase() === clean.toLowerCase())) {
    selectedTags.value = normalizeTags([...selectedTags.value, clean]);
    mergeTagCatalog([clean]);
    if (noteMode.value === "preview") noteMode.value = "edit";
  }
}

function setReviewQuestions(items) {
  if (!noteInsights.value) {
    noteInsights.value = { flashcards: [] };
  }
  noteInsights.value = {
    ...noteInsights.value,
    flashcards: Array.isArray(items) ? items : []
  };
}

function openNewReviewQuestionForm() {
  reviewQuestionForm.value = { id: 0, question: "", answer: "" };
  reviewQuestionError.value = "";
  reviewQuestionFormOpen.value = true;
}

function editReviewQuestion(card) {
  reviewQuestionForm.value = {
    id: Number(card?.id || 0),
    question: String(card?.question || ""),
    answer: String(card?.answer || "")
  };
  reviewQuestionError.value = "";
  reviewQuestionFormOpen.value = true;
}

function closeReviewQuestionForm() {
  reviewQuestionFormOpen.value = false;
  reviewQuestionError.value = "";
}

async function saveReviewQuestion() {
  const noteID = Number(selectedId.value || 0);
  const question = String(reviewQuestionForm.value.question || "").trim();
  const answer = String(reviewQuestionForm.value.answer || "").trim();
  if (!noteID || !question) {
    reviewQuestionError.value = "先写一个复习问题。";
    return;
  }
  reviewQuestionSaving.value = true;
  reviewQuestionError.value = "";
  try {
    const id = Number(reviewQuestionForm.value.id || 0);
    const saved = await api(
      id ? `/api/notes/${noteID}/review-questions/${id}` : `/api/notes/${noteID}/review-questions`,
      {
        method: id ? "PUT" : "POST",
        body: JSON.stringify({ question, answer })
      }
    );
    const current = insightFlashcards.value || [];
    const next = id
      ? current.map((item) => Number(item.id) === id ? saved : item)
      : [saved, ...current];
    setReviewQuestions(next);
    closeReviewQuestionForm();
  } catch (err) {
    reviewQuestionError.value = err?.message || "保存失败";
  } finally {
    reviewQuestionSaving.value = false;
  }
}

async function deleteReviewQuestion(card) {
  const noteID = Number(selectedId.value || 0);
  const id = Number(card?.id || 0);
  if (!noteID || !id) return;
  reviewQuestionDeletingID.value = id;
  reviewQuestionError.value = "";
  try {
    await api(`/api/notes/${noteID}/review-questions/${id}`, { method: "DELETE" });
    setReviewQuestions((insightFlashcards.value || []).filter((item) => Number(item.id) !== id));
    if (Number(reviewQuestionForm.value.id || 0) === id) closeReviewQuestionForm();
  } catch (err) {
    reviewQuestionError.value = err?.message || "删除失败";
  } finally {
    reviewQuestionDeletingID.value = 0;
  }
}

async function generateReviewQuestions() {
  const noteID = Number(selectedId.value || 0);
  if (!noteID) return;
  reviewQuestionGenerating.value = true;
  reviewQuestionError.value = "";
  try {
    const res = await api(`/api/notes/${noteID}/review-questions/generate`, {
      method: "POST",
      body: JSON.stringify({ count: 3 })
    });
    setReviewQuestions(res.items || res.created || []);
    reviewQuestionFormOpen.value = false;
  } catch (err) {
    reviewQuestionError.value = err?.message || "AI 生成失败";
  } finally {
    reviewQuestionGenerating.value = false;
  }
}

function askFromFlashcard(card) {
  aiInput.value = `${card.question}\n\n请结合当前笔记回答。`;
  aiOpen.value = true;
}

function pruneAIThreadsMap(mapLike) {
  const now = Date.now();
  const next = new Map();
  for (const [key, entry] of mapLike instanceof Map ? mapLike.entries() : []) {
    const updatedAt = Number(entry?.updatedAt || 0);
    const messages = Array.isArray(entry?.messages) ? entry.messages : [];
    if (!updatedAt || now-updatedAt > AI_THREAD_TTL_MS || messages.length === 0) continue;
    next.set(key, {
      updatedAt,
      messages: messages.map((msg) => ({
        role: msg?.role === "user" ? "user" : "assistant",
        content: String(msg?.content || "")
      }))
    });
  }
  return next;
}

function loadAIThreads() {
  const raw = localStorage.getItem(AI_THREAD_KEY);
  if (!raw) {
    aiThreads.value = new Map();
    aiMessages.value = [];
    return;
  }
  try {
    const parsed = JSON.parse(raw);
    const map = new Map();
    for (const [key, entry] of Object.entries(parsed || {})) {
      map.set(String(key), {
        updatedAt: Number(entry?.updatedAt || 0),
        messages: Array.isArray(entry?.messages) ? entry.messages : []
      });
    }
    aiThreads.value = pruneAIThreadsMap(map);
  } catch {
    aiThreads.value = new Map();
  }
  syncAIThreadMessages();
}

function saveAIThreads() {
  const pruned = pruneAIThreadsMap(aiThreads.value);
  aiThreads.value = pruned;
  const obj = {};
  for (const [key, entry] of pruned.entries()) {
    obj[key] = entry;
  }
  localStorage.setItem(AI_THREAD_KEY, JSON.stringify(obj));
}

function readAIThreadMessages(key) {
  const entry = aiThreads.value.get(key);
  if (!entry) return [];
  if (Date.now()-Number(entry.updatedAt || 0) > AI_THREAD_TTL_MS) {
    aiThreads.value.delete(key);
    saveAIThreads();
    return [];
  }
  return (entry.messages || []).map((msg) => ({
    role: msg?.role === "user" ? "user" : "assistant",
    content: String(msg?.content || "")
  }));
}

function writeAIThreadMessages(key, messages) {
  const clean = (messages || [])
    .map((msg) => ({
      role: msg?.role === "user" ? "user" : "assistant",
      content: String(msg?.content || "")
    }))
    .filter((msg) => msg.content);
  if (clean.length === 0) {
    aiThreads.value.delete(key);
  } else {
    aiThreads.value.set(key, {
      updatedAt: Date.now(),
      messages: clean
    });
  }
  saveAIThreads();
}

function syncAIThreadMessages() {
  aiMessages.value = readAIThreadMessages(aiThreadKey.value);
}

function speechRecognitionCtor() {
  if (typeof window === "undefined") return null;
  return window.SpeechRecognition || window.webkitSpeechRecognition || null;
}

function speechErrorText(code) {
  switch (String(code || "")) {
    case "not-allowed":
    case "service-not-allowed":
      return "语音输入权限被拒绝，请允许浏览器使用麦克风。";
    case "network":
      return "语音识别网络异常，请稍后重试。";
    case "no-speech":
      return "没有识别到语音，请再说一遍。";
    case "audio-capture":
      return "没有检测到可用的麦克风设备。";
    case "aborted":
      return "";
    default:
      return "语音输入已中断，请重试。";
  }
}

function ensureSpeechRecognition() {
  if (speechRecognitionRef.value) return speechRecognitionRef.value;
  const Ctor = speechRecognitionCtor();
  if (!Ctor) return null;
  const recognition = new Ctor();
  recognition.lang = "zh-CN";
  recognition.continuous = true;
  recognition.interimResults = true;
  recognition.maxAlternatives = 1;
  recognition.onresult = (event) => {
    let finalText = "";
    let interimText = "";
    for (let i = event.resultIndex; i < event.results.length; i += 1) {
      const transcript = Array.from(event.results[i] || [])
        .map((item) => item?.transcript || "")
        .join("");
      if (!transcript) continue;
      if (event.results[i].isFinal) {
        finalText += transcript;
      } else {
        interimText += transcript;
      }
    }
    if (finalText) voiceTranscript.value += finalText;
    voiceInterim.value = interimText;
  };
  recognition.onerror = (event) => {
    const msg = speechErrorText(event?.error);
    if (msg) voiceError.value = msg;
  };
  recognition.onend = () => {
    if (voiceFinishPending.value) {
      finishVoiceInputImmediately();
      voiceFinishPending.value = false;
      return;
    }
    if (voicePausePending.value) {
      voicePausePending.value = false;
      voiceState.value = "paused";
      return;
    }
    if (voiceState.value === "listening") {
      voiceState.value = "paused";
    }
  };
  speechRecognitionRef.value = recognition;
  return recognition;
}

function normalizeVoiceText(raw) {
  return String(raw || "")
    .replace(/\s+\n/g, "\n")
    .replace(/\n{3,}/g, "\n\n")
    .replace(/[ \t]{2,}/g, " ")
    .trim();
}

function insertVoiceText(raw) {
  const text = normalizeVoiceText(raw);
  if (!text) return;
  flushEditorHistory();
  const area = editorTextareaRef.value;
  const current = String(markdown.value || "");
  const hasSelection =
    area &&
    Number.isInteger(area.selectionStart) &&
    Number.isInteger(area.selectionEnd);
  const start = hasSelection ? area.selectionStart : current.length;
  const end = hasSelection ? area.selectionEnd : current.length;
  const before = current.slice(0, start);
  const after = current.slice(end);
  const prefix = before && !before.endsWith("\n") ? "\n\n" : "";
  const suffix = after && !after.startsWith("\n") ? "\n\n" : "";
  const nextValue = `${before}${prefix}${text}${suffix}${after}`;
  markdown.value = nextValue;
  onMarkdownInput();
  pushEditorHistory(true);
  nextTick(() => {
    if (!area) return;
    const pos = (before + prefix + text).length;
    area.focus();
    area.setSelectionRange(pos, pos);
  });
}

function clearVoiceBuffers() {
  voiceTranscript.value = "";
  voiceInterim.value = "";
}

function finishVoiceInputImmediately() {
  insertVoiceText(`${voiceTranscript.value}\n${voiceInterim.value}`);
  clearVoiceBuffers();
  voiceState.value = "idle";
}

function resetVoiceInput() {
  voicePausePending.value = false;
  voiceFinishPending.value = false;
  voiceState.value = "idle";
  clearVoiceBuffers();
  voiceError.value = "";
  const recognition = speechRecognitionRef.value;
  if (!recognition) return;
  recognition.onresult = null;
  recognition.onerror = null;
  recognition.onend = null;
  try {
    recognition.abort();
  } catch {
    // noop
  }
  speechRecognitionRef.value = null;
}

function startVoiceInput() {
  voiceError.value = "";
  const recognition = ensureSpeechRecognition();
  if (!recognition) {
    voiceError.value = "当前浏览器不支持语音输入。";
    return;
  }
  if (voiceState.value === "idle") {
    clearVoiceBuffers();
  }
  voicePausePending.value = false;
  voiceFinishPending.value = false;
  try {
    recognition.start();
    voiceState.value = "listening";
  } catch (err) {
    voiceError.value = `语音输入启动失败：${err?.message || err}`;
  }
}

function pauseVoiceInput() {
  if (voiceState.value !== "listening") return;
  voicePausePending.value = true;
  voiceFinishPending.value = false;
  const recognition = speechRecognitionRef.value;
  if (!recognition) {
    voiceState.value = "paused";
    return;
  }
  try {
    recognition.stop();
  } catch {
    voicePausePending.value = false;
    voiceState.value = "paused";
  }
}

function finishVoiceInput() {
  voiceError.value = "";
  if (voiceState.value === "listening") {
    voiceFinishPending.value = true;
    voicePausePending.value = false;
    try {
      speechRecognitionRef.value?.stop();
      return;
    } catch {
      // fallback to immediate insert below
    }
  }
  finishVoiceInputImmediately();
}

function normalizeTags(list) {
  const seen = new Set();
  const out = [];
  for (const raw of list || []) {
    const tag = String(raw || "").trim();
    if (!tag) continue;
    const key = tag.toLowerCase();
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(tag);
  }
  return out;
}

function mergeTagCatalog(tags) {
  allTags.value = normalizeTags([...(allTags.value || []), ...(tags || [])]);
}

function hasTag(tag) {
  return selectedTagSet.value.has(String(tag || "").toLowerCase());
}

function toggleTag(tag) {
  const t = String(tag || "").trim();
  if (!t) return;
  if (hasTag(t)) {
    selectedTags.value = (selectedTags.value || []).filter((x) => String(x).toLowerCase() !== t.toLowerCase());
    return;
  }
  selectedTags.value = normalizeTags([...(selectedTags.value || []), t]);
}

function removeTag(tag) {
  const t = String(tag || "").trim();
  if (!t) return;
  selectedTags.value = (selectedTags.value || []).filter((x) => String(x).toLowerCase() !== t.toLowerCase());
}

function createTagFromQuery() {
  if (!tagQueryTrimmed.value) return;
  const t = tagQueryTrimmed.value;
  mergeTagCatalog([t]);
  if (!hasTag(t)) {
    selectedTags.value = normalizeTags([...(selectedTags.value || []), t]);
  }
  tagQuery.value = "";
}

async function deleteTagGlobally(tag) {
  const t = String(tag || "").trim();
  if (!t) return;
  const ok = window.confirm(`删除标签「${t}」后，它会从所有笔记中移除。继续吗？`);
  if (!ok) return;
  await api(`/api/tags?tag=${encodeURIComponent(t)}`, { method: "DELETE" });
  allTags.value = (allTags.value || []).filter((x) => String(x).toLowerCase() !== t.toLowerCase());
  selectedTags.value = (selectedTags.value || []).filter((x) => String(x).toLowerCase() !== t.toLowerCase());
  await Promise.all([loadNotes(), loadAllNotes(), loadArchived(), loadTags()]);
  if (selectedId.value) {
    await openNote(selectedId.value, { push: false, track: false });
  }
}

function closeTagMenu() {
  tagMenuOpen.value = false;
  tagQuery.value = "";
}

function onTagQueryKeydown(e) {
  if (e.key === "Enter") {
    e.preventDefault();
    createTagFromQuery();
  }
}

function toggleSidebarPin() {
  if (sidebarPinned.value) {
    sidebarPinned.value = false;
    sidebarPeek.value = false;
    return;
  }
  sidebarPinned.value = true;
  sidebarPeek.value = false;
}

function openSidebarPeek() {
  if (sidebarPinned.value) return;
  clearTimeout(sidebarPeekCloseTimer.value);
  sidebarPeek.value = true;
}

function closeSidebarPeek() {
  if (sidebarPinned.value) return;
  clearTimeout(sidebarPeekCloseTimer.value);
  sidebarPeekCloseTimer.value = setTimeout(() => {
    sidebarPeek.value = false;
  }, 120);
}

function isFolder(note) {
  const tags = Array.isArray(note?.tags) ? note.tags.map((t) => String(t).toLowerCase()) : [];
  return tags.includes("folder");
}

function iconForNote(note) {
  return isFolder(note) ? "[文件夹]" : "[笔记]";
}

function noteIconClass(note) {
  return isFolder(note) ? "folder" : "note";
}

function coverClass(id) {
  const idx = Number(id || 0) % 6;
  return `cover-${idx}`;
}

function formatDate(raw) {
  if (!raw) return "";
  return new Date(raw).toLocaleDateString();
}

function formatRecentDate(raw) {
  if (!raw) return "";
  return new Date(raw).toLocaleDateString("zh-CN", {
    year: "numeric",
    month: "numeric",
    day: "numeric"
  });
}

function summarizeLine(raw, max = 36) {
  const text = String(raw || "").replace(/\s+/g, " ").trim();
  if (text.length <= max) return text;
  return `${text.slice(0, Math.max(1, max - 1))}...`;
}

function timeBucket(raw) {
  const d = new Date(raw);
  const now = new Date();
  const diff = now.getTime() - d.getTime();
  const day = Math.floor(diff / (24 * 3600 * 1000));
  if (day <= 0) return "today";
  if (day <= 30) return "30d";
  return "older";
}

function containsFold(text, query) {
  const t = String(text || "").toLowerCase();
  const q = String(query || "").trim().toLowerCase();
  if (!q) return true;
  return t.includes(q);
}

function countFold(text, query) {
  const t = String(text || "").toLowerCase();
  const q = String(query || "").trim().toLowerCase();
  if (!q || !t) return 0;
  let idx = 0;
  let count = 0;
  while (idx < t.length) {
    const hit = t.indexOf(q, idx);
    if (hit < 0) break;
    count += 1;
    idx = hit + q.length;
  }
  return count;
}

function scoreSearchResult(note, query) {
  const q = String(query || "").trim();
  if (!q) return 0;
  const title = String(note?.title || "");
  const body = String(note?.markdown || "");
  const titleCount = countFold(title, q);
  const bodyCount = countFold(body, q);
  const titleStarts = title.toLowerCase().startsWith(q.toLowerCase()) ? 1 : 0;
  return titleStarts * 80 + titleCount * 24 + bodyCount * 8;
}

function escapeHTML(raw) {
  return String(raw || "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

function highlightFirst(raw, query) {
  const text = String(raw || "");
  const q = String(query || "").trim();
  if (!q) return escapeHTML(text);
  const lower = text.toLowerCase();
  const hit = lower.indexOf(q.toLowerCase());
  if (hit < 0) return escapeHTML(text);
  const pre = escapeHTML(text.slice(0, hit));
  const mid = escapeHTML(text.slice(hit, hit + q.length));
  const post = escapeHTML(text.slice(hit + q.length));
  return `${pre}<mark>${mid}</mark>${post}`;
}

function searchTitleHTML(note) {
  const titleText = note?.title || "未命名";
  return highlightFirst(titleText, searchText.value);
}

function searchSnippetHTML(note) {
  const q = String(searchText.value || "").trim();
  if (!q) return "";
  const md = String(note?.markdown || "");
  if (!md) return "";
  const lines = md
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);
  const match = lines.find((line) => line.toLowerCase().includes(q.toLowerCase()));
  if (!match) return "";
  return highlightFirst(match.slice(0, 120), q);
}

function nearestFolderIDForNote(note) {
  if (!note) return null;
  if (isFolder(note)) return Number(note.id);
  const byID = notesByID.value;
  let cur = note.parent_id ? byID.get(note.parent_id) || null : null;
  const seen = new Set();
  while (cur && !seen.has(cur.id)) {
    if (isFolder(cur)) return Number(cur.id);
    seen.add(cur.id);
    cur = cur.parent_id ? byID.get(cur.parent_id) || null : null;
  }
  return null;
}

function setSelectedFolderByNote(note) {
  selectedFolderID.value = nearestFolderIDForNote(note);
}

function notePath(note) {
  const byID = notesByID.value;
  const chain = [];
  let cur = note;
  const seen = new Set();
  while (cur && cur.parent_id && !seen.has(cur.parent_id)) {
    seen.add(cur.parent_id);
    const p = byID.get(cur.parent_id);
    if (!p) break;
    if (isFolder(p)) {
      chain.push(p.title || `未命名 #${p.id}`);
    }
    cur = p;
  }
  if (!chain.length) return "根目录";
  return chain.reverse().join(" / ");
}

function noteFullPath(note) {
  if (!note) return "";
  const parts = [];
  const byID = notesByID.value;
  let cur = note;
  const seen = new Set();
  while (cur && !seen.has(cur.id)) {
    seen.add(cur.id);
    parts.push(cur.title || `未命名 #${cur.id}`);
    if (!cur.parent_id) break;
    const parent = byID.get(cur.parent_id) || null;
    if (!parent || !isFolder(parent)) break;
    cur = parent;
  }
  return parts.reverse().join("/");
}

function folderSelectLabel(note) {
  const path = noteFullPath(note);
  if (!path) return note?.title || "未命名文件夹";
  return path;
}

function normalizeNotePath(raw) {
  return String(raw || "")
    .split("/")
    .map((part) => decodeURIComponent(String(part || "").trim()))
    .filter(Boolean)
    .join("/")
    .toLowerCase();
}

function resolveNoteHref(targetRaw) {
  const target = String(targetRaw || "").trim();
  if (!target || target.startsWith("#")) return target;
  if (target.startsWith("note://")) return target;
  if (/^[a-zA-Z][a-zA-Z\d+.-]*:/.test(target)) return target;
  const key = normalizeNotePath(target);
  if (!key) return target;
  const id = notePathMap.value.get(key);
  return id ? `note://${id}` : target;
}

function resolvePathLinks(md) {
  return String(md || "").replace(/(!?)\[([^\]]+)\]\(([^)]+)\)/g, (all, bang, label, target) => {
    if (bang) return all;
    return `[${label}](${resolveNoteHref(target)})`;
  });
}

function resolveInternalLinks(md) {
  return resolvePathLinks(resolveWikiLinks(md));
}

function readRouteNoteID() {
  const u = new URL(window.location.href);
  const raw = u.searchParams.get("note");
  if (!raw) return null;
  const id = Number(raw);
  return Number.isFinite(id) && id > 0 ? id : null;
}

function writeRoute(noteID, replace = false) {
  const u = new URL(window.location.href);
  if (noteID) u.searchParams.set("note", String(noteID));
  else u.searchParams.delete("note");
  const state = noteID ? { note: noteID } : { home: true };
  if (replace) {
    window.history.replaceState(state, "", u);
  } else {
    window.history.pushState(state, "", u);
  }
}

function loadRecentVisits() {
  const raw = localStorage.getItem(VISIT_KEY);
  if (!raw) {
    recentVisits.value = [];
    return;
  }
  try {
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) {
      recentVisits.value = [];
      return;
    }
    recentVisits.value = parsed
      .map((x) => ({ id: Number(x.id), at: String(x.at || "") }))
      .filter((x) => x.id > 0)
      .slice(0, 40);
  } catch {
    recentVisits.value = [];
  }
}

function saveRecentVisits() {
  localStorage.setItem(VISIT_KEY, JSON.stringify(recentVisits.value.slice(0, 40)));
}

function loadFavorites() {
  const raw = localStorage.getItem(FAVORITE_KEY);
  if (!raw) {
    favoriteIDs.value = [];
    return;
  }
  try {
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) {
      favoriteIDs.value = [];
      return;
    }
    const seen = new Set();
    favoriteIDs.value = parsed
      .map((id) => Number(id))
      .filter((id) => Number.isFinite(id) && id > 0)
      .filter((id) => {
        if (seen.has(id)) return false;
        seen.add(id);
        return true;
      })
      .slice(0, 80);
  } catch {
    favoriteIDs.value = [];
  }
}

function saveFavorites() {
  localStorage.setItem(FAVORITE_KEY, JSON.stringify((favoriteIDs.value || []).slice(0, 80)));
}

function setFavorite(id, value) {
  const num = Number(id);
  if (!num) return;
  const next = (favoriteIDs.value || []).filter((x) => x !== num);
  if (value) next.unshift(num);
  favoriteIDs.value = next.slice(0, 80);
  saveFavorites();
}

function toggleFavorite(id) {
  const num = Number(id);
  if (!num) return;
  setFavorite(num, !favoriteSet.value.has(num));
}

function onFavoriteAction() {
  if (!selectedId.value) return;
  toggleFavorite(selectedId.value);
}

function trackVisit(id) {
  const now = new Date().toISOString();
  const filtered = (recentVisits.value || []).filter((x) => x.id !== id);
  filtered.unshift({ id, at: now });
  recentVisits.value = filtered.slice(0, 40);
  saveRecentVisits();
}

async function loadNotes() {
  notes.value = await api("/api/notes?lite=1");
}

async function loadAllNotes() {
  allNotes.value = await api("/api/notes?include_archived=1&lite=1");
}

async function loadArchived() {
  archivedNotes.value = await api("/api/notes?archived=1&lite=1");
}

async function loadTags() {
  try {
    const tags = await api("/api/tags");
    allTags.value = normalizeTags(tags || []);
  } catch {
    allTags.value = [];
  }
}

function patchNoteInLists(note) {
  const patch = (arr) => {
    const idx = arr.findIndex((n) => n.id === note.id);
    if (idx >= 0) {
      arr[idx] = { ...arr[idx], ...note };
      return;
    }
    arr.unshift(note);
  };
  patch(notes.value);
  patch(allNotes.value);
  if (String(note.markdown || "").trim() || String(note.html || "").trim()) {
    noteDetailCache.value.set(note.id, note);
  }
}

function resolveWikiLinks(md) {
  const byTitle = new Map((allNotes.value || []).map((n) => [String(n.title || "").trim().toLowerCase(), n.id]));
  return String(md || "").replace(/\[\[([^\]]+)\]\]/g, (all, name) => {
    const key = String(name || "").trim().toLowerCase();
    const id = byTitle.get(key);
    if (!id) return all;
    return `[${name}](note://${id})`;
  });
}

async function getMermaid() {
  if (!mermaidLoader) {
    mermaidLoader = import("mermaid").then((module) => module.default);
  }
  const mermaid = await mermaidLoader;
  if (!mermaidConfigured) {
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: "strict",
      theme: "dark",
      themeVariables: {
        background: "#1d2027",
        primaryColor: "#2d3646",
        primaryTextColor: "#f4f6fb",
        primaryBorderColor: "#6f8fc8",
        lineColor: "#98a9c6",
        secondaryColor: "#243c3b",
        tertiaryColor: "#2a2d33",
        fontFamily: "Inter, system-ui, sans-serif"
      }
    });
    mermaidConfigured = true;
  }
  return mermaid;
}

function isMermaidCodeBlock(code) {
  const className = String(code.className || "");
  if (/\b(?:language|lang)-mermaid\b/i.test(className)) return true;
  const firstLine = String(code.textContent || "")
    .split(/\r?\n/)
    .map((line) => line.trim())
    .find(Boolean);
  return Boolean(firstLine && mermaidStartRE.test(firstLine));
}

function scheduleMermaidRender() {
  const token = ++mermaidRenderToken;
  nextTick(() => {
    renderMermaidDiagrams(token);
  });
}

async function renderMermaidDiagrams(token = ++mermaidRenderToken) {
  await nextTick();
  if (token !== mermaidRenderToken) return;

  const blocks = Array.from(document.querySelectorAll(".preview pre > code")).filter(isMermaidCodeBlock);
  if (blocks.length === 0) return;
  const mermaid = await getMermaid();
  if (token !== mermaidRenderToken) return;

  for (let i = 0; i < blocks.length; i += 1) {
    if (token !== mermaidRenderToken) return;
    const code = blocks[i];
    const pre = code.closest("pre");
    if (!pre || pre.dataset.mermaidRendered === "true") continue;

    const source = String(code.textContent || "").trim();
    if (!source) continue;
    pre.dataset.mermaidRendered = "true";

    try {
      const id = `mermaid-${Date.now()}-${i}`;
      const { svg, bindFunctions } = await mermaid.render(id, source);
      if (token !== mermaidRenderToken) return;
      const container = document.createElement("div");
      container.className = "mermaid-diagram";
      container.innerHTML = svg;
      pre.replaceWith(container);
      bindFunctions?.(container);
    } catch (err) {
      pre.dataset.mermaidRendered = "false";
      pre.classList.add("mermaid-error");
      pre.title = err instanceof Error ? err.message : "Mermaid render failed";
    }
  }
}

async function renderPreview() {
  const resolved = resolveInternalLinks(markdown.value);
  const res = await api("/api/render", {
    method: "POST",
    body: JSON.stringify({ markdown: resolved })
  });
  previewHTML.value = res.html || "";
  scheduleMermaidRender();
}

function onMarkdownInput() {
  clearTimeout(previewTimer.value);
  previewTimer.value = setTimeout(() => {
    renderPreview();
  }, 180);
}

function applySelectedNote(note) {
  clearTimeout(autosaveTimer.value);
  clearTimeout(historyTimer.value);
  resetVoiceInput();
  aiOptimizeMessage.value = "";
  hydrating.value = true;
  activeView.value = "note";
  noteMode.value = "preview";
  selectedId.value = note.id;
  selectedNote.value = note;
  title.value = note.title || "";
  markdown.value = note.markdown || "";
  parentID.value = note.parent_id ? String(note.parent_id) : "";
  selectedTags.value = normalizeTags(note.tags || []);
  noteInsights.value = null;
  intelligenceMessage.value = "";
  intelligenceLoading.value = false;
  reviewQuestionFormOpen.value = false;
  reviewQuestionError.value = "";
  mergeTagCatalog(selectedTags.value);
  setSelectedFolderByNote(note);
  previewHTML.value = note.html || "";
  lastSavedSignature.value = noteSignature(note);
  saveState.value = "saved";
  resetEditorHistory({
    title: title.value,
    markdown: markdown.value,
    parent_id: parentID.value ? Number(parentID.value) : null,
    tags: selectedTags.value
  });
  nextTick(() => {
    hydrating.value = false;
  });
  void loadCachedNoteIntelligence(note.id);
}

function noteSignature(note) {
  return JSON.stringify({
    title: note.title || "未命名",
    markdown: note.markdown || "",
    parent_id: note.parent_id || null,
    tags: Array.isArray(note.tags) ? note.tags : []
  });
}

function payloadSignature(payload) {
  return JSON.stringify(payload);
}

function editorSnapshot() {
  return {
    title: title.value || "",
    markdown: markdown.value || "",
    parent_id: parentID.value ? Number(parentID.value) : null,
    tags: normalizeTags(selectedTags.value)
  };
}

function editorSnapshotSignature(snapshot) {
  return JSON.stringify({
    title: snapshot.title || "",
    markdown: snapshot.markdown || "",
    parent_id: snapshot.parent_id || null,
    tags: Array.isArray(snapshot.tags) ? snapshot.tags : []
  });
}

function cloneEditorSnapshot(snapshot) {
  return {
    title: snapshot.title || "",
    markdown: snapshot.markdown || "",
    parent_id: snapshot.parent_id || null,
    tags: Array.isArray(snapshot.tags) ? [...snapshot.tags] : []
  };
}

function captureActiveEditorSelection() {
  const active = document.activeElement;
  if (active === titleInputRef.value) {
    return {
      field: "title",
      start: active.selectionStart ?? null,
      end: active.selectionEnd ?? null
    };
  }
  if (active === editorTextareaRef.value) {
    return {
      field: "markdown",
      start: active.selectionStart ?? null,
      end: active.selectionEnd ?? null
    };
  }
  return null;
}

function restoreEditorSelection(selection) {
  if (!selection) return;
  const el = selection.field === "title" ? titleInputRef.value : editorTextareaRef.value;
  if (!el) return;
  const value = String(el.value || "");
  const start = Math.max(0, Math.min(Number(selection.start ?? value.length), value.length));
  const end = Math.max(start, Math.min(Number(selection.end ?? start), value.length));
  el.focus();
  if (typeof el.setSelectionRange === "function") {
    el.setSelectionRange(start, end);
  }
}

function resetEditorHistory(snapshot = null) {
  const base = cloneEditorSnapshot(snapshot || editorSnapshot());
  undoStack.value = [base];
  redoStack.value = [];
  lastHistorySignature.value = editorSnapshotSignature(base);
}

function pushEditorHistory(force = false) {
  if (!selectedId.value || activeView.value !== "note" || noteMode.value !== "edit" || hydrating.value || historyApplying.value) {
    return;
  }
  const snapshot = cloneEditorSnapshot(editorSnapshot());
  const signature = editorSnapshotSignature(snapshot);
  if (!force && signature === lastHistorySignature.value) {
    return;
  }
  const stack = undoStack.value || [];
  if (stack.length > 0 && editorSnapshotSignature(stack[stack.length - 1]) === signature) {
    lastHistorySignature.value = signature;
    return;
  }
  undoStack.value = [...stack, snapshot].slice(-120);
  redoStack.value = [];
  lastHistorySignature.value = signature;
}

function flushEditorHistory() {
  clearTimeout(historyTimer.value);
  historyTimer.value = null;
  pushEditorHistory(false);
}

function scheduleEditorHistory() {
  if (!selectedId.value || activeView.value !== "note" || noteMode.value !== "edit" || hydrating.value || historyApplying.value) {
    return;
  }
  clearTimeout(historyTimer.value);
  historyTimer.value = setTimeout(() => {
    pushEditorHistory(false);
  }, 240);
}

function applyEditorSnapshot(snapshot) {
  const nextSnapshot = cloneEditorSnapshot(snapshot);
  const selection = captureActiveEditorSelection();
  historyApplying.value = true;
  title.value = nextSnapshot.title || "";
  markdown.value = nextSnapshot.markdown || "";
  parentID.value = nextSnapshot.parent_id ? String(nextSnapshot.parent_id) : "";
  selectedTags.value = normalizeTags(nextSnapshot.tags || []);
  mergeTagCatalog(selectedTags.value);
  saveState.value = "dirty";
  renderPreview().catch(() => {
    // keep current preview if render fails
  });
  nextTick(() => {
    historyApplying.value = false;
    restoreEditorSelection(selection);
  });
}

function undoEditor() {
  flushEditorHistory();
  const stack = undoStack.value || [];
  if (stack.length <= 1) return;
  const current = stack[stack.length - 1];
  const previous = stack[stack.length - 2];
  redoStack.value = [cloneEditorSnapshot(current), ...(redoStack.value || [])].slice(0, 120);
  undoStack.value = stack.slice(0, -1);
  lastHistorySignature.value = editorSnapshotSignature(previous);
  applyEditorSnapshot(previous);
}

function redoEditor() {
  flushEditorHistory();
  const stack = redoStack.value || [];
  if (stack.length === 0) return;
  const nextSnapshot = cloneEditorSnapshot(stack[0]);
  redoStack.value = stack.slice(1);
  undoStack.value = [...(undoStack.value || []), nextSnapshot].slice(-120);
  lastHistorySignature.value = editorSnapshotSignature(nextSnapshot);
  applyEditorSnapshot(nextSnapshot);
}

function buildPayload() {
  return {
    title: title.value.trim() || "未命名",
    markdown: markdown.value,
    parent_id: parentID.value ? Number(parentID.value) : null,
    tags: normalizeTags(selectedTags.value)
  };
}

async function openHome(options = {}) {
  const { push = true } = options;
  activeView.value = "home";
  noteMode.value = "preview";
  selectedId.value = null;
  selectedNote.value = null;
  noteInsights.value = null;
  saveState.value = "idle";
  if (push) writeRoute(null, false);
  void loadHomeIntelligence();
}

async function refreshNoteFromServer(id) {
  try {
    const latest = await api(`/api/notes/${id}`);
    noteDetailCache.value.set(latest.id, latest);
    patchNoteInLists(latest);
    if (selectedId.value === id && saveState.value === "saved") {
      applySelectedNote(latest);
    }
  } catch {
    // ignore refresh failures
  }
}

async function openNote(id, options = {}) {
  const { push = true, track = true } = options;
  const num = Number(id);
  if (!num) return;

  const lite = notesByID.value.get(num) || (notes.value || []).find((n) => n.id === num) || null;
  if (lite && isFolder(lite)) {
    selectedFolderID.value = lite.id;
    folderOpen.value[lite.id] = true;
    await openHome({ push });
    return;
  }

  if (selectedId.value === num && activeView.value === "note") {
    if (push) writeRoute(num, false);
    return;
  }

  if (push) writeRoute(num, false);

  const cachedFull = noteDetailCache.value.get(num) || null;
  if (cachedFull) {
    applySelectedNote(cachedFull);
    if (track) trackVisit(num);
    void refreshNoteFromServer(num);
    return;
  }

  const cachedLite = lite;
  if (cachedLite && (cachedLite.markdown || cachedLite.html)) {
    applySelectedNote(cachedLite);
    if (track) trackVisit(num);
    void refreshNoteFromServer(num);
    return;
  }

  const note = await api(`/api/notes/${num}`);
  if (isFolder(note)) {
    selectedFolderID.value = note.id;
    folderOpen.value[note.id] = true;
    await openHome({ push });
    return;
  }
  noteDetailCache.value.set(note.id, note);
  patchNoteInLists(note);
  applySelectedNote(note);
  if (track) trackVisit(num);
}

function isDescendantOf(ancestorID, nodeID) {
  const byID = notesByID.value;
  let cur = byID.get(Number(nodeID)) || null;
  const seen = new Set();
  while (cur && cur.parent_id && !seen.has(cur.id)) {
    if (Number(cur.parent_id) === Number(ancestorID)) return true;
    seen.add(cur.id);
    cur = byID.get(cur.parent_id) || null;
  }
  return false;
}

async function updateNoteFields(id, patch = {}) {
  const num = Number(id);
  if (!num) return null;
  const current = await api(`/api/notes/${num}`);
  const payload = {
    title: patch.title ?? current.title ?? "未命名",
    markdown: patch.markdown ?? current.markdown ?? "",
    parent_id: patch.parent_id !== undefined ? patch.parent_id : current.parent_id ?? null,
    tags: patch.tags ?? current.tags ?? []
  };
  const updated = await api(`/api/notes/${num}`, { method: "PUT", body: JSON.stringify(payload) });
  noteDetailCache.value.set(updated.id, updated);
  patchNoteInLists(updated);
  return updated;
}

async function moveNoteToParent(noteID, parentID) {
  const id = Number(noteID);
  const targetParent = parentID ? Number(parentID) : null;
  if (!id) return;
  if (targetParent && (targetParent === id || isDescendantOf(id, targetParent))) return;
  if (targetParent) {
    const parent = notesByID.value.get(targetParent) || null;
    if (!parent || !isFolder(parent)) return;
  }
  const cur = notesByID.value.get(id) || null;
  const oldParent = cur?.parent_id ? Number(cur.parent_id) : null;
  if (oldParent === targetParent) return;
  await updateNoteFields(id, { parent_id: targetParent });
  if (targetParent) folderOpen.value[targetParent] = true;
  await Promise.all([loadNotes(), loadAllNotes()]);
  if (selectedId.value === id) {
    await openNote(id, { push: false, track: false });
  }
}

async function createPage(parent = null, folder = false) {
  const targetParent =
    parent !== null && parent !== undefined
      ? Number(parent)
      : selectedFolderID.value
        ? Number(selectedFolderID.value)
        : null;
  const payload = {
    title: folder ? "新文件夹" : "新页面",
    markdown: folder
      ? "# 新文件夹\n\n用于归类页面。你可以在这里记录目录说明，并在其下创建子页面。"
      : "# 新页面\n\n在这里开始写作。支持标准 Markdown。",
    parent_id: targetParent || null,
    tags: folder ? ["folder"] : []
  };
  const res = await api("/api/notes", { method: "POST", body: JSON.stringify(payload) });
  const note = res.note || res;
  await Promise.all([loadNotes(), loadAllNotes()]);
  if (targetParent) {
    folderOpen.value[targetParent] = true;
  }
  await openNote(note.id);
  noteMode.value = "edit";
  nextTick(() => {
    document.querySelector(".title-input")?.focus();
  });
}

function newPageFromTop() {
  createPage(null, false);
}

function newFolderUnderRoot() {
  createPage(null, true);
}

function toggleFolder(id) {
  folderOpen.value[id] = folderOpen.value[id] === false;
}

async function onPrivateRowClick(note) {
  if (!note) return;
  if (isFolder(note)) {
    selectedFolderID.value = note.id;
    folderOpen.value[note.id] = true;
    await openHome();
    return;
  }
  await openNote(note.id);
}

function onRowDragStart(row, e) {
  const id = Number(row?.note?.id || 0);
  if (!id) return;
  draggingNoteID.value = id;
  dragOverNoteID.value = null;
  if (e?.dataTransfer) {
    e.dataTransfer.effectAllowed = "move";
    e.dataTransfer.setData("text/plain", String(id));
  }
}

function onRowDragOver(row) {
  const dragID = Number(draggingNoteID.value || 0);
  const targetID = Number(row?.note?.id || 0);
  if (!dragID || !targetID || dragID === targetID) {
    dragOverNoteID.value = null;
    return;
  }
  dragOverNoteID.value = targetID;
}

async function onRowDrop(row) {
  const dragID = Number(draggingNoteID.value || 0);
  const target = row?.note || null;
  dragOverNoteID.value = null;
  draggingNoteID.value = null;
  if (!dragID || !target) return;
  const parentID = isFolder(target) ? target.id : null;
  await moveNoteToParent(dragID, parentID);
}

async function onRootDrop() {
  const dragID = Number(draggingNoteID.value || 0);
  dragOverNoteID.value = null;
  draggingNoteID.value = null;
  if (!dragID) return;
  await moveNoteToParent(dragID, null);
}

function onRowDragEnd() {
  dragOverNoteID.value = null;
  draggingNoteID.value = null;
}

function openContextMenu(e, note) {
  const id = Number(note?.id || 0);
  if (!id) return;
  const menuWidth = 280;
  const menuHeight = 270;
  contextMenuX.value = Math.min(e.clientX, window.innerWidth - menuWidth - 10);
  contextMenuY.value = Math.min(e.clientY, window.innerHeight - menuHeight - 10);
  contextNoteID.value = id;
  contextMenuOpen.value = true;
}

function closeContextMenu() {
  contextMenuOpen.value = false;
  contextNoteID.value = null;
}

async function renameFromContext() {
  const note = contextNote.value;
  if (!note) return;
  const nextTitle = window.prompt("重命名页面", note.title || "未命名");
  if (nextTitle === null) return;
  const clean = String(nextTitle).trim();
  if (!clean) return;
  await updateNoteFields(note.id, { title: clean });
  await Promise.all([loadNotes(), loadAllNotes()]);
  if (selectedId.value === note.id) {
    await openNote(note.id, { push: false, track: false });
  }
  closeContextMenu();
}

async function moveFromContext() {
  const note = contextNote.value;
  if (!note) return;
  const folders = (activeNotes.value || []).filter((n) => isFolder(n) && n.id !== note.id);
  const lines = folders.map((n, idx) => `${idx + 1}. ${n.title || `未命名 #${n.id}`}`).join("\n");
  const input = window.prompt(
    `移动到：输入序号，0 表示根目录。\n${lines || "（当前没有可用文件夹）"}`,
    "0"
  );
  if (input === null) return;
  const picked = Number(String(input).trim());
  if (!Number.isFinite(picked) || picked < 0 || picked > folders.length) return;
  const targetParent = picked === 0 ? null : folders[picked - 1].id;
  await moveNoteToParent(note.id, targetParent);
  closeContextMenu();
}

async function archiveFromContext() {
  const note = contextNote.value;
  if (!note) return;
  await api(`/api/notes/${note.id}/archive`, {
    method: "PATCH",
    body: JSON.stringify({ value: true })
  });
  setFavorite(note.id, false);
  await Promise.all([loadNotes(), loadAllNotes(), loadArchived()]);
  if (selectedId.value === note.id) {
    await openHome();
  }
  closeContextMenu();
}

function openInNewTabFromContext() {
  const note = contextNote.value;
  if (!note) return;
  const u = new URL(window.location.href);
  u.searchParams.set("note", String(note.id));
  window.open(u.toString(), "_blank", "noopener");
  closeContextMenu();
}

function toggleFavoriteFromContext() {
  const note = contextNote.value;
  if (!note) return;
  toggleFavorite(note.id);
  closeContextMenu();
}

async function saveNote(silent = false) {
  if (isSaving.value) return null;
  const payload = buildPayload();
  const method = selectedId.value ? "PUT" : "POST";
  const path = selectedId.value ? `/api/notes/${selectedId.value}` : "/api/notes";
  if (silent) saveState.value = "saving";
  isSaving.value = true;
  try {
    const res = await api(path, { method, body: JSON.stringify(payload) });
    const note = res.note || res;
    patchNoteInLists(note);
    selectedTags.value = normalizeTags(note.tags || selectedTags.value);
    mergeTagCatalog(note.tags || []);
    if (!selectedId.value) {
      await Promise.all([loadNotes(), loadAllNotes()]);
      await openNote(note.id);
      return note;
    }
    selectedNote.value = note;
    previewHTML.value = note.html || "";
    lastSavedSignature.value = noteSignature(note);
    saveState.value = "saved";
    void loadHomeIntelligence();
    if (!silent) {
      await Promise.all([loadNotes(), loadAllNotes()]);
    }
    return note;
  } catch (err) {
    if (silent) saveState.value = "error";
    throw err;
  } finally {
    isSaving.value = false;
  }
}

function scheduleAutosave() {
  if (!selectedId.value || hydrating.value || activeView.value !== "note") return;
  const sig = payloadSignature(buildPayload());
  if (sig === lastSavedSignature.value) {
    saveState.value = "saved";
    return;
  }
  saveState.value = "dirty";
  clearTimeout(autosaveTimer.value);
  autosaveTimer.value = setTimeout(async () => {
    if (!selectedId.value || hydrating.value) return;
    const latest = payloadSignature(buildPayload());
    if (latest === lastSavedSignature.value) return;
    try {
      await saveNote(true);
    } catch {
      // handled by state
    }
  }, 1200);
}

async function deleteNote() {
  if (!selectedId.value) return;
  await api(`/api/notes/${selectedId.value}`, { method: "DELETE" });
  await Promise.all([loadNotes(), loadAllNotes(), loadArchived()]);
  await openHome();
}

async function toggleArchive() {
  if (!selectedId.value || !selectedNote.value) return;
  const note = await api(`/api/notes/${selectedId.value}/archive`, {
    method: "PATCH",
    body: JSON.stringify({ value: !selectedNote.value.is_archived })
  });
  await Promise.all([loadNotes(), loadAllNotes(), loadArchived()]);
  if (note.is_archived) {
    await openHome();
    return;
  }
  await openNote(note.id);
}

async function setCurrentNoteStatus(status) {
  if (!selectedId.value) return;
  const clean = status === "completed" ? "completed" : "unfinished";
  const updated = await api(`/api/notes/${selectedId.value}/status`, {
    method: "PATCH",
    body: JSON.stringify({ status: clean })
  });
  noteDetailCache.value.set(updated.id, updated);
  patchNoteInLists(updated);
  selectedNote.value = updated;
  await Promise.all([loadNotes(), loadAllNotes(), loadWorkspaceDashboard()]);
}

async function duplicateNote() {
  if (!selectedId.value) return;
  const res = await api(`/api/notes/${selectedId.value}/duplicate`, { method: "POST" });
  const note = res.note || res;
  await Promise.all([loadNotes(), loadAllNotes()]);
  await openNote(note.id);
  noteMode.value = "edit";
}

function extractFilename(contentDisposition) {
  const raw = String(contentDisposition || "");
  if (!raw) return "";
  const utf8 = raw.match(/filename\*=UTF-8''([^;]+)/i);
  if (utf8?.[1]) {
    try {
      return decodeURIComponent(utf8[1]);
    } catch {
      return utf8[1];
    }
  }
  const plain = raw.match(/filename=\"?([^\";]+)\"?/i);
  return plain?.[1] || "";
}

async function exportMarkdown() {
  if (!selectedId.value) return;
  const res = await fetch(`/api/notes/${selectedId.value}/export.md`);
  if (!res.ok) {
    const txt = await res.text();
    throw new Error(txt || `Export failed: ${res.status}`);
  }
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = extractFilename(res.headers.get("Content-Disposition")) || `note-${selectedId.value}.md`;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

function openSearchModal() {
  openHome();
  searchModalOpen.value = true;
  searchCursor.value = -1;
  if (!searchText.value.trim() && normalizeTags(searchTags.value || []).length === 0) {
    searchResults.value = (recentCards.value || []).map((x) => x.note).slice(0, 40);
  } else {
    runSearch();
  }
}

function closeSearchModal() {
  searchModalOpen.value = false;
  searchCursor.value = -1;
  searchTagMenuOpen.value = false;
}

async function runSearch() {
  const q = searchText.value.trim();
  const tags = normalizeTags(searchTags.value || []);
  if (!q && tags.length === 0) {
    searchResults.value = (recentCards.value || []).map((x) => x.note).slice(0, 40);
    searchCursor.value = searchResults.value.length > 0 ? 0 : -1;
    return;
  }
  searchLoading.value = true;
  try {
    const params = new URLSearchParams();
    params.set("include_archived", "1");
    if (q) params.set("q", q);
    const list = await api(`/api/notes?${params.toString()}`);
    searchResults.value = list || [];
    searchCursor.value = (searchResults.value || []).length > 0 ? 0 : -1;
  } finally {
    searchLoading.value = false;
  }
}

function cycleSearchSort() {
  const order = ["relevance", "updated_desc", "updated_asc", "title_asc"];
  const idx = order.indexOf(searchSort.value);
  searchSort.value = order[(idx + 1) % order.length];
}

function cycleSearchDate() {
  const order = ["all", "today", "30d", "older"];
  const idx = order.indexOf(searchDate.value);
  searchDate.value = order[(idx + 1) % order.length];
}

function toggleSearchOnlyTitle() {
  const next = !searchOnlyTitle.value;
  searchOnlyTitle.value = next;
  if (next) searchInPage.value = false;
}

function toggleSearchInPage() {
  const next = !searchInPage.value;
  searchInPage.value = next;
  if (next) searchOnlyTitle.value = false;
}

function onSearchInputKeydown(e) {
  const list = flatSearchResults.value || [];
  if (e.key === "ArrowDown") {
    e.preventDefault();
    if (!list.length) return;
    searchCursor.value = searchCursor.value < 0 ? 0 : (searchCursor.value + 1) % list.length;
    return;
  }
  if (e.key === "ArrowUp") {
    e.preventDefault();
    if (!list.length) return;
    if (searchCursor.value <= 0) {
      searchCursor.value = list.length - 1;
    } else {
      searchCursor.value -= 1;
    }
    return;
  }
  if (e.key === "Enter") {
    e.preventDefault();
    const idx = Number(searchCursor.value);
    const note = idx >= 0 && idx < list.length ? list[idx] : list[0];
    if (note) selectSearchResult(note);
  }
}

function onSearchModalInput() {
  clearTimeout(searchModalTimer.value);
  searchModalTimer.value = setTimeout(() => {
    runSearch();
  }, 220);
}

function toggleSearchTag(tag) {
  const next = String(tag || "").trim();
  if (!next) {
    searchTags.value = [];
  } else {
    const current = normalizeTags(searchTags.value || []);
    const exists = current.some((item) => String(item).toLowerCase() === next.toLowerCase());
    searchTags.value = exists
      ? current.filter((item) => String(item).toLowerCase() !== next.toLowerCase())
      : [...current, next];
  }
  if (!searchModalOpen.value) return;
  runSearch();
}

function clearSearchTags() {
  searchTags.value = [];
  if (!searchModalOpen.value) return;
  runSearch();
}

async function selectSearchResult(note) {
  closeSearchModal();
  await openNote(note.id);
}

function openTrashModal() {
  trashModalOpen.value = true;
  loadArchived();
}

function closeTrashModal() {
  trashModalOpen.value = false;
}

async function restoreFromTrash(note) {
  await api(`/api/notes/${note.id}/archive`, {
    method: "PATCH",
    body: JSON.stringify({ value: false })
  });
  await Promise.all([loadNotes(), loadAllNotes(), loadArchived()]);
}

async function deletePermanently(note) {
  if (!window.confirm(`确定永久删除《${note.title || "未命名"}》吗？`)) return;
  await api(`/api/notes/${note.id}`, { method: "DELETE" });
  await Promise.all([loadNotes(), loadAllNotes(), loadArchived()]);
}

function onPreviewClick(e) {
  const a = e.target?.closest?.("a");
  if (!a) return;
  const href = resolveNoteHref(a.getAttribute("href") || "");
  if (!href.startsWith("note://")) return;
  e.preventDefault();
  const id = Number(href.slice("note://".length));
  if (id > 0) openNote(id);
}

function enterEditMode() {
  if (!selectedId.value) return;
  noteMode.value = "edit";
  resetEditorHistory();
}

function exitEditMode() {
  if (voiceState.value !== "idle" || voiceTranscript.value || voiceInterim.value) {
    finishVoiceInputImmediately();
    resetVoiceInput();
  }
  noteMode.value = "preview";
  closeTagMenu();
}

function scrollRecent(dir) {
  const el = recentStripRef.value;
  if (!el) return;
  el.scrollBy({ left: dir * 360, behavior: "smooth" });
  setTimeout(() => updateRecentNavState(), 220);
}

function updateRecentNavState() {
  const el = recentStripRef.value;
  if (!el) {
    canRecentLeft.value = false;
    canRecentRight.value = false;
    return;
  }
  const left = el.scrollLeft > 3;
  const right = el.scrollLeft + el.clientWidth < el.scrollWidth - 3;
  canRecentLeft.value = left;
  canRecentRight.value = right;
}

function scrollFavorite(dir) {
  const el = favoriteStripRef.value;
  if (!el) return;
  el.scrollBy({ left: dir * 360, behavior: "smooth" });
  setTimeout(() => updateFavoriteNavState(), 220);
}

function updateFavoriteNavState() {
  const el = favoriteStripRef.value;
  if (!el) {
    canFavoriteLeft.value = false;
    canFavoriteRight.value = false;
    return;
  }
  const left = el.scrollLeft > 3;
  const right = el.scrollLeft + el.clientWidth < el.scrollWidth - 3;
  canFavoriteLeft.value = left;
  canFavoriteRight.value = right;
}

function stripCitationMarks(answer) {
  return String(answer || "")
    .replace(/\s*\[\d+\]/g, "")
    .replace(/\n{3,}/g, "\n\n")
    .trim();
}

function citationIndexes(answer) {
  const out = [];
  const regex = /\[(\d+)\]/g;
  let m;
  while ((m = regex.exec(String(answer || ""))) !== null) {
    const n = Number(m[1]);
    if (!Number.isFinite(n) || n <= 0) continue;
    out.push(n - 1);
  }
  return Array.from(new Set(out));
}

function contextTitles(contexts, indexes) {
  const ids = [];
  if (indexes.length > 0) {
    for (const idx of indexes) {
      const c = contexts[idx];
      if (!c || !c.note_id) continue;
      ids.push(c.note_id);
    }
  } else {
    for (const c of contexts || []) {
      if (!c?.note_id) continue;
      ids.push(c.note_id);
      if (ids.length >= 3) break;
    }
  }
  const uniq = Array.from(new Set(ids));
  return uniq.map((id) => notesByID.value.get(id)?.title || `笔记 #${id}`);
}

async function askAI() {
  const assistantMode = activeView.value === "assistant";
  const contextText = assistantMode ? assistantScopedContextText() : "";
  const q = aiInput.value.trim() || (contextText ? "请总结我添加的上下文，并给出可执行建议。" : "");
  if (!q || aiLoading.value) return;
  const threadKey = aiThreadKey.value;
  const noteID = Number(selectedId.value || 0);
  const sendQuery = contextText
    ? `请优先参考以下用户添加的上下文回答。\n\n${contextText}\n\n用户问题：${q}`
    : q;
  aiInput.value = "";
  aiLoading.value = true;
  const messages = readAIThreadMessages(threadKey);
  messages.push({ role: "user", content: q });
  const reply = { role: "assistant", content: "正在思考..." };
  messages.push(reply);
  writeAIThreadMessages(threadKey, messages);
  if (aiThreadKey.value === threadKey) {
    aiMessages.value = messages.map((msg) => ({ ...msg }));
    scrollAIToBottom();
  }
  try {
    const res = await api("/api/rag/ask", {
      method: "POST",
      body: JSON.stringify({
        query: sendQuery,
        top_k: assistantMode ? 7 : 5,
        note_id: noteID || null,
        mode: assistantMode ? assistantScope.value : "rag"
      })
    });
    const rawAnswer = String(res.answer || "").trim();
    const cleaned = stripCitationMarks(rawAnswer);
    const indexes = citationIndexes(rawAnswer);
    const refs = contextTitles(res.contexts || [], indexes);
    if (refs.length > 0) {
      reply.content = `${cleaned || "模型没有返回内容。"}\n\n参考页面：${refs.join("、")}`;
    } else {
      reply.content = cleaned || "模型没有返回内容。";
    }
  } catch (err) {
    reply.content = `请求失败：${err.message || err}`;
  } finally {
    writeAIThreadMessages(threadKey, messages);
    if (aiThreadKey.value === threadKey) {
      aiMessages.value = messages.map((msg) => ({ ...msg }));
    }
    aiLoading.value = false;
    scrollAIToBottom();
  }
}

async function optimizeWithAI() {
  if (!selectedId.value || aiOptimizing.value) return;
  aiOptimizeMessage.value = "";
  aiOptimizing.value = true;
  try {
    flushEditorHistory();
    const res = await api(`/api/notes/${selectedId.value}/optimize`, {
      method: "POST",
      body: JSON.stringify({
        title: title.value.trim() || "未命名",
        markdown: markdown.value
      })
    });
    const nextMarkdown = String(res.markdown || "").trim();
    if (!nextMarkdown) {
      throw new Error("AI 没有返回可用内容");
    }
    markdown.value = nextMarkdown;
    previewHTML.value = res.html || previewHTML.value;
    saveState.value = "dirty";
    pushEditorHistory(true);
    const refs = Array.isArray(res.references) ? res.references : [];
    aiOptimizeMessage.value =
      refs.length > 0
        ? `AI 已结合 ${refs.length} 篇关联笔记完成排版优化。`
        : "AI 已完成当前笔记排版优化。";
  } catch (err) {
    aiOptimizeMessage.value = `AI 优化失败：${err?.message || err}`;
  } finally {
    aiOptimizing.value = false;
  }
}

function applyQuickMemoTemplate(tpl, text) {
  const cleanText = String(text || "").trim();
  const base = String(tpl?.markdown || "").trim();
  if (!base) return `# AI 速记\n\n${cleanText}\n`;
  if (/\{\{\s*(content|voice|transcript|速记内容)\s*\}\}/i.test(base)) {
    return base.replace(/\{\{\s*(content|voice|transcript|速记内容)\s*\}\}/gi, cleanText);
  }
  return `${base}\n\n## AI 速记内容\n\n${cleanText}\n`;
}

function startQuickMemoVoice() {
  quickMemoError.value = "";
  quickMemoResult.value = null;
  const Ctor = speechRecognitionCtor();
  if (!Ctor) {
    quickMemoError.value = "当前浏览器不支持语音识别，可以直接在文本框里输入速记内容。";
    return;
  }
  if (!quickMemoRecognitionRef.value) {
    const recognition = new Ctor();
    recognition.lang = "zh-CN";
    recognition.continuous = true;
    recognition.interimResults = true;
    recognition.maxAlternatives = 1;
    recognition.onresult = (event) => {
      let finalText = "";
      let interimText = "";
      for (let i = event.resultIndex; i < event.results.length; i += 1) {
        const transcript = Array.from(event.results[i] || [])
          .map((item) => item?.transcript || "")
          .join("");
        if (!transcript) continue;
        if (event.results[i].isFinal) finalText += transcript;
        else interimText += transcript;
      }
      if (finalText) quickMemoText.value = normalizeVoiceText(`${quickMemoText.value}\n${finalText}`);
      quickMemoInterim.value = interimText;
    };
    recognition.onerror = (event) => {
      const msg = speechErrorText(event?.error);
      if (msg) quickMemoError.value = msg;
    };
    recognition.onend = () => {
      if (quickMemoFinishPending.value) {
        quickMemoFinishPending.value = false;
        quickMemoState.value = "idle";
        quickMemoInterim.value = "";
        void createQuickMemoNote();
        return;
      }
      if (quickMemoState.value === "listening") quickMemoState.value = "paused";
    };
    quickMemoRecognitionRef.value = recognition;
  }
  try {
    quickMemoRecognitionRef.value.start();
    quickMemoState.value = "listening";
  } catch (err) {
    quickMemoError.value = `语音识别启动失败：${err?.message || err}`;
  }
}

function pauseQuickMemoVoice() {
  if (quickMemoState.value !== "listening") return;
  try {
    quickMemoRecognitionRef.value?.stop();
  } catch {
    quickMemoState.value = "paused";
  }
}

function finishQuickMemoVoice() {
  quickMemoText.value = normalizeVoiceText(`${quickMemoText.value}\n${quickMemoInterim.value}`);
  quickMemoInterim.value = "";
  if (quickMemoState.value === "listening") {
    quickMemoFinishPending.value = true;
    try {
      quickMemoRecognitionRef.value?.stop();
      return;
    } catch {
      quickMemoFinishPending.value = false;
    }
  }
  quickMemoState.value = "idle";
  void createQuickMemoNote();
}

async function createQuickMemoNote() {
  const text = normalizeVoiceText(quickMemoText.value);
  const tpl = quickMemoTemplate.value;
  if (!text || quickMemoSaving.value) return;
  quickMemoSaving.value = true;
  quickMemoResult.value = null;
  try {
    const title = `AI速记 ${new Date().toLocaleString("zh-CN", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit" })}`;
    const note = await api("/api/notes", {
      method: "POST",
      body: JSON.stringify({
        title,
        markdown: applyQuickMemoTemplate(tpl, text),
        parent_id: quickMemoParentID.value ? Number(quickMemoParentID.value) : null,
        tags: normalizeTags(tpl?.tags || [])
      })
    });
    const created = note.note || note;
    quickMemoResult.value = created;
    quickMemoText.value = "";
    await Promise.all([loadNotes(), loadAllNotes(), loadHomeIntelligence(), loadTags()]);
  } catch (err) {
    quickMemoError.value = `AI 速记保存失败：${err?.message || err}`;
  } finally {
    quickMemoSaving.value = false;
  }
}

function scrollAIToBottom() {
  nextTick(() => {
    const el = aiMessagesRef.value;
    if (!el) return;
    el.scrollTop = el.scrollHeight;
  });
}

function onAIKeydown(e) {
  if (e.key === "Enter" && !e.shiftKey) {
    e.preventDefault();
    askAI();
  }
}

function isEditorHistoryTarget(target) {
  return !!target?.closest?.(".editor-view");
}

const onKeydown = (e) => {
  const lowerKey = e.key.toLowerCase();
  if ((e.ctrlKey || e.metaKey) && activeView.value === "note" && noteMode.value === "edit" && isEditorHistoryTarget(e.target)) {
    if (lowerKey === "z" && !e.shiftKey) {
      e.preventDefault();
      undoEditor();
      return;
    }
    if ((lowerKey === "z" && e.shiftKey) || lowerKey === "y") {
      e.preventDefault();
      redoEditor();
      return;
    }
  }
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") {
    e.preventDefault();
    if (activeView.value === "note" && noteMode.value === "edit") saveNote(false);
    return;
  }
  const calendarShortcut = activeView.value === "calendar" && (
    (isMacLike.value && e.metaKey && lowerKey === "k") ||
    (!isMacLike.value && e.altKey && lowerKey === "k")
  );
  if (calendarShortcut) {
    e.preventDefault();
    openCalendarCommand();
    return;
  }
  if (calendarContextMenuOpen.value && lowerKey === "c") {
    e.preventDefault();
    createPlanFromContext();
    return;
  }
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") {
    e.preventDefault();
    openSearchModal();
    return;
  }
  if (e.key === "Escape") {
    if (researchHistoryOpen.value) researchHistoryOpen.value = false;
    if (recommendHistoryOpen.value) recommendHistoryOpen.value = false;
    if (calendarContextMenuOpen.value) closeCalendarContextMenu();
    if (calendarCommandOpen.value) closeCalendarCommand();
    if (calendarPlanSearchOpen.value) closeCalendarPlanSearch();
    if (guideModalOpen.value) closeGuide();
    if (importModalOpen.value) closeImportModal();
    if (templateModalOpen.value) closeTemplateModal();
    if (searchModalOpen.value) closeSearchModal();
    if (trashModalOpen.value) closeTrashModal();
    if (contextMenuOpen.value) closeContextMenu();
    if (tagMenuOpen.value) closeTagMenu();
  }
};

const onDocumentPointerDown = (e) => {
  if (contextMenuOpen.value && !e.target?.closest?.(".context-menu")) {
    closeContextMenu();
  }
  if (calendarContextMenuOpen.value && !e.target?.closest?.(".calendar-context-menu")) {
    closeCalendarContextMenu();
  }
  if (tagMenuOpen.value && !e.target?.closest?.(".tag-select")) {
    closeTagMenu();
  }
  if (searchTagMenuOpen.value && !e.target?.closest?.(".search-tag-menu")) {
    searchTagMenuOpen.value = false;
  }
  if (importTagMenuOpen.value && !e.target?.closest?.(".import-tag-picker")) {
    importTagMenuOpen.value = false;
  }
  if (templateTagMenuOpen.value && !e.target?.closest?.(".template-tag-picker")) {
    templateTagMenuOpen.value = false;
  }
  if (researchHistoryOpen.value && !e.target?.closest?.(".research-history-menu")) {
    researchHistoryOpen.value = false;
  }
  if (recommendHistoryOpen.value && !e.target?.closest?.(".recommend-history-menu")) {
    recommendHistoryOpen.value = false;
  }
};

const onPopState = async () => {
  const id = readRouteNoteID();
  if (id) {
    await openNote(id, { push: false, track: false });
    return;
  }
  await openHome({ push: false });
};

onMounted(async () => {
  loadRecentVisits();
  loadFavorites();
  loadAIThreads();
  loadPlanTasks();
  loadTemplatePrefs();
  await Promise.all([loadNotes(), loadAllNotes(), loadArchived(), loadTags()]);
  void loadHomeIntelligence();
  const id = readRouteNoteID();
  if (id) {
    await openNote(id, { push: false, track: false });
    writeRoute(id, true);
  } else {
    await openHome({ push: false });
    writeRoute(null, true);
  }
  document.addEventListener("keydown", onKeydown);
  document.addEventListener("pointerdown", onDocumentPointerDown);
  window.addEventListener("popstate", onPopState);
  window.addEventListener("resize", updateRecentNavState);
  window.addEventListener("resize", updateFavoriteNavState);
  nextTick(() => updateRecentNavState());
  nextTick(() => updateFavoriteNavState());
});

onUnmounted(() => {
  resetVoiceInput();
  try {
    quickMemoRecognitionRef.value?.abort();
  } catch {
    // noop
  }
  document.removeEventListener("keydown", onKeydown);
  document.removeEventListener("pointerdown", onDocumentPointerDown);
  window.removeEventListener("popstate", onPopState);
  window.removeEventListener("resize", updateRecentNavState);
  window.removeEventListener("resize", updateFavoriteNavState);
  clearTimeout(searchTimer.value);
  clearTimeout(previewTimer.value);
  clearTimeout(autosaveTimer.value);
  clearTimeout(searchModalTimer.value);
  clearTimeout(sidebarPeekCloseTimer.value);
  clearTimeout(historyTimer.value);
});

watch([title, markdown, parentID, () => selectedTags.value.join("\u0001")], () => {
  if (hydrating.value || activeView.value !== "note" || noteMode.value !== "edit") return;
  scheduleAutosave();
});

watch([title, markdown, parentID, () => selectedTags.value.join("\u0001")], () => {
  if (hydrating.value || activeView.value !== "note" || noteMode.value !== "edit" || historyApplying.value) return;
  scheduleEditorHistory();
});

watch(previewHTML, () => {
  scheduleMermaidRender();
});

watch(noteMode, () => {
  scheduleMermaidRender();
});

watch(searchModalOpen, (v) => {
  if (!v) return;
  nextTick(() => {
    searchInputRef.value?.focus();
  });
});

watch(calendarPlanSearchOpen, (v) => {
  if (!v) return;
  nextTick(() => {
    calendarPlanSearchInputRef.value?.focus();
  });
});

watch(calendarCommandQuery, () => {
  calendarCommandCursor.value = 0;
});

watch(calendarCommands, (list) => {
  if (calendarCommandCursor.value >= list.length) {
    calendarCommandCursor.value = Math.max(0, list.length - 1);
  }
});

watch(calendarPlanSearchQuery, () => {
  calendarPlanSearchCursor.value = 0;
});

watch(calendarPlanSearchResults, (list) => {
  if (calendarPlanSearchCursor.value >= list.length) {
    calendarPlanSearchCursor.value = Math.max(0, list.length - 1);
  }
});

watch(aiOpen, (v) => {
  if (v) scrollAIToBottom();
});

watch(aiThreadKey, () => {
  syncAIThreadMessages();
  if (aiOpen.value) scrollAIToBottom();
});

watch(recentCards, () => {
  nextTick(() => updateRecentNavState());
});

watch(favoriteCards, () => {
  nextTick(() => updateFavoriteNavState());
});

watch(searchGroups, () => {
  const size = flatSearchResults.value.length;
  if (size <= 0) {
    searchCursor.value = -1;
    return;
  }
  if (searchCursor.value < 0) {
    searchCursor.value = 0;
    return;
  }
  if (searchCursor.value >= size) {
    searchCursor.value = size - 1;
  }
});

watch(activeNotes, (list) => {
  const byID = new Map((list || []).map((n) => [n.id, n]));
  const curFolder = selectedFolderID.value ? byID.get(Number(selectedFolderID.value)) : null;
  if (!curFolder || !isFolder(curFolder)) {
    selectedFolderID.value = null;
  }

  const alive = new Set((list || []).map((n) => n.id));
  const filtered = (favoriteIDs.value || []).filter((id) => alive.has(id));
  if (filtered.length !== favoriteIDs.value.length) {
    favoriteIDs.value = filtered;
    saveFavorites();
  }
});

watch(workspaceTemplates, (list) => {
  if (quickMemoTemplateKey.value || !(list || []).length) return;
  quickMemoTemplateKey.value = list[0].key;
}, { immediate: true });
</script>

<template>
  <div class="shell" :class="{ 'rail-mode': !sidebarPinned, 'sidebar-peek': !sidebarPinned && sidebarPeek }">
    <div
      v-if="!sidebarPinned"
      class="sidebar-rail"
      @mouseenter="openSidebarPeek"
      @mouseleave="closeSidebarPeek"
    >
      <button class="rail-btn" @click="toggleSidebarPin">=</button>
    </div>

    <aside class="sidebar" @mouseenter="openSidebarPeek" @mouseleave="closeSidebarPeek">
      <div class="workspace">
        <div class="avatar">N</div>
        <div class="workspace-text">
          <strong>{{ workspaceName }}</strong>
          <small>本地工作区</small>
        </div>
        <div class="workspace-actions">
          <button class="icon-btn icon-text-btn" @click="toggleSidebarPin">{{ sidebarPinned ? "收起" : "固定" }}</button>
          <button class="icon-btn icon-text-btn" @click="newPageFromTop">新建</button>
        </div>
      </div>

      <div class="top-menu">
        <button class="nav-item" @click="openSearchModal">
          <span class="nav-label">搜索</span>
        </button>
        <button class="nav-item" :class="{ active: activeView === 'home' }" @click="openHome">
          <span class="nav-label">主页</span>
        </button>
        <button class="nav-item" :class="{ active: activeView === 'workspace' }" @click="openWorkspace">
          <span class="nav-label">智能知识工作台</span>
        </button>
        <button class="nav-item" :class="{ active: activeView === 'calendar' }" @click="openToolView('calendar')">
          <span class="nav-label">日历</span>
        </button>
        <button class="nav-item" :class="{ active: activeView === 'assistant' }" @click="openAssistantView">
          <span class="nav-label">AI 助手</span>
        </button>
      </div>

      <div class="private-panel">
        <div class="private-head">
          <button class="ghost-btn title-btn title-toggle" @click="privateExpanded = !privateExpanded">
            <span>私人</span>
            <span class="caret" :class="{ open: privateExpanded }">></span>
          </button>
          <div class="private-actions">
            <button class="icon-btn icon-text-btn" @click="newFolderUnderRoot">文件夹</button>
            <button class="icon-btn icon-text-btn" @click="newPageFromTop">新建</button>
          </div>
        </div>

        <div v-if="privateExpanded" class="private-list" @dragover.prevent @drop.prevent="onRootDrop">
          <button
            v-for="row in privateRows"
            :key="row.note.id"
            class="private-row"
            :class="{
              active: selectedId === row.note.id || (isFolder(row.note) && selectedFolderID === row.note.id),
              'drag-over': dragOverNoteID === row.note.id
            }"
            :style="{ paddingLeft: `${10 + row.depth * 16}px` }"
            draggable="true"
            @click="onPrivateRowClick(row.note)"
            @contextmenu.prevent="openContextMenu($event, row.note)"
            @dragstart="onRowDragStart(row, $event)"
            @dragover.prevent="onRowDragOver(row)"
            @drop.prevent.stop="onRowDrop(row)"
            @dragend="onRowDragEnd"
          >
            <span
              class="row-chev"
              :class="{ open: row.expanded, placeholder: !row.hasChildren }"
              @click.stop="row.hasChildren && toggleFolder(row.note.id)"
            >
              >
            </span>
            <span class="row-icon" :class="noteIconClass(row.note)"></span>
            <span class="row-title">{{ row.note.title || "未命名" }}</span>
            <span v-if="isFolder(row.note)" class="row-add" @click.stop="createPage(row.note.id, false)">新建</span>
          </button>
        </div>
      </div>
    </aside>

    <main class="content">
      <section v-if="activeView === 'home'" class="home-view">
        <h1>{{ greeting }}</h1>
<div class="recent-wrap" @mouseenter="recentHover = true" @mouseleave="recentHover = false">
          <div class="recent-header">
            <span>最近访问</span>
          </div>
          <div class="recent-strip-box">
            <button class="recent-nav left" :class="{ show: recentHover && canRecentLeft }" @click="scrollRecent(-1)">&lt;</button>
            <div ref="recentStripRef" class="recent-strip" @scroll="updateRecentNavState">
              <button
                v-for="item in recentCards"
                :key="item.note.id"
                class="card"
                :class="coverClass(item.note.id)"
                @click="openNote(item.note.id)"
                @contextmenu.prevent="openContextMenu($event, item.note)"
              >
                <div class="card-cover"></div>
                <div class="card-body">
                  <strong>{{ item.note.title || "未命名" }}</strong>
                  <small>{{ formatRecentDate(item.visitedAt || item.note.updated_at) }}</small>
                </div>
              </button>
            </div>
            <button class="recent-nav right" :class="{ show: recentHover && canRecentRight }" @click="scrollRecent(1)">&gt;</button>
          </div>
        </div>

        <div
          class="recent-wrap favorite-wrap"
          @mouseenter="favoriteHover = true"
          @mouseleave="favoriteHover = false"
        >
          <div class="recent-header">
            <span>收藏</span>
          </div>
          <div class="recent-strip-box">
            <button class="recent-nav left" :class="{ show: favoriteHover && canFavoriteLeft }" @click="scrollFavorite(-1)">&lt;</button>
            <div ref="favoriteStripRef" class="recent-strip" @scroll="updateFavoriteNavState">
              <button
                v-for="item in favoriteCards"
                :key="`fav-${item.note.id}`"
                class="card"
                :class="coverClass(item.note.id)"
                @click="openNote(item.note.id)"
                @contextmenu.prevent="openContextMenu($event, item.note)"
              >
                <div class="card-cover"></div>
                <div class="card-body">
                  <strong>{{ item.note.title || "未命名" }}</strong>
                  <small>{{ formatRecentDate(item.favoritedAt || item.note.updated_at) }}</small>
                </div>
              </button>
            </div>
            <button class="recent-nav right" :class="{ show: favoriteHover && canFavoriteRight }" @click="scrollFavorite(1)">&gt;</button>
          </div>
        </div>

        <section class="dashboard-charts lower-charts">
          <div class="chart-panel">
            <div class="panel-head"><strong>笔记完成状态</strong><span>手动标注</span></div>
            <div class="pie-chart-wrap">
              <svg viewBox="0 0 42 42" class="pie-chart">
                <circle cx="21" cy="21" r="15.9" fill="transparent" stroke="#edf0f4" stroke-width="6"></circle>
                <circle
                  v-for="(item, index) in dashboardNotePie"
                  :key="`note-pie-${item.label}`"
                  cx="21"
                  cy="21"
                  r="15.9"
                  fill="transparent"
                  :stroke="index === 0 ? '#f59e0b' : '#16a34a'"
                  stroke-width="6"
                  :stroke-dasharray="pieDashArray(item, dashboardNotePie)"
                  :stroke-dashoffset="pieDashOffset(index, dashboardNotePie)"
                ></circle>
              </svg>
              <div class="chart-legend">
                <span v-for="item in dashboardNotePie" :key="`note-legend-${item.label}`">{{ item.label }} {{ item.value }}</span>
              </div>
            </div>
          </div>
          <div class="chart-panel wide">
            <div class="panel-head"><strong>笔记概览</strong><span>柱状图</span></div>
            <div class="bar-chart">
              <div v-for="item in dashboardBars" :key="`bar-${item.label}`" class="bar-item">
                <span>{{ item.label }}</span>
                <div><em :style="{ width: `${Math.max(4, (Number(item.value || 0) / chartMax(dashboardBars)) * 100)}%` }"></em></div>
                <strong>{{ item.value }}</strong>
              </div>
            </div>
          </div>
          <div class="chart-panel full">
            <div class="panel-head"><strong>14 天笔记趋势</strong><span>更新记录</span></div>
            <svg viewBox="0 0 300 130" class="line-chart">
              <polyline :points="lineChartPoints(dashboardNoteTrend)" fill="none" stroke="#16a34a" stroke-width="3" />
            </svg>
            <div class="chart-legend horizontal"><span>绿色：笔记更新</span></div>
          </div>
        </section>
      </section>

      <section v-else-if="activeView === 'assistant'" class="assistant-page" :class="{ chatting: aiMessages.length }">
        <button class="assistant-history-toggle" title="历史记录" @click="aiHistoryOpen = !aiHistoryOpen">
          <span class="clock-icon"></span>
        </button>

        <aside v-if="aiHistoryOpen" class="assistant-history-panel">
          <div class="assistant-history-head">
            <strong>历史记录</strong>
            <button class="ghost-btn" @click="startNewAssistantThread">新会话</button>
          </div>
          <div
            v-for="item in assistantHistoryItems"
            :key="item.key"
            class="assistant-history-item"
            :class="{ active: item.key === assistantThreadKey }"
            @click="selectAssistantHistory(item.key)"
          >
            <div class="assistant-history-main">
              <span>{{ item.title }}</span>
              <small>{{ item.preview }}</small>
              <em>{{ item.updatedAt }}</em>
            </div>
            <button
              type="button"
              class="assistant-history-delete"
              title="删除历史"
              aria-label="删除历史"
              @click.stop="deleteAssistantHistory(item.key)"
            >
              x
            </button>
          </div>
          <p v-if="!assistantHistoryItems.length" class="muted">暂无历史记录。</p>
        </aside>

        <div class="assistant-stage" :class="{ chatting: aiMessages.length }">
          <div v-if="!aiMessages.length" class="assistant-brand">
            <div class="assistant-logo">AI</div>
            <h1>今天需要我帮你处理什么？</h1>
          </div>

          <div v-if="aiMessages.length" ref="aiMessagesRef" class="assistant-chat-log">
            <div v-for="(m, idx) in aiMessages" :key="`assistant-${idx}`" class="ai-msg" :class="m.role">
              <small>{{ m.role === "user" ? "你" : "AI" }}</small>
              <div class="ai-bubble">{{ m.content }}</div>
            </div>
          </div>

          <div class="assistant-composer">
            <input
              ref="assistantAttachmentFileInputRef"
              class="hidden-file-input"
              type="file"
              multiple
              accept=".md,.markdown,.txt,.csv,.json,.log"
              @change="onAssistantFileChange"
            />
            <div v-if="assistantAttachments.length" class="assistant-attachments">
              <span
                v-for="(item, index) in assistantAttachments"
                :key="`${item.type}-${item.id}-${index}`"
                class="assistant-attachment-chip"
              >
                {{ item.type === "note" ? "笔记" : "文件" }}：{{ item.name }}
                <button type="button" @click="removeAssistantAttachment(index)">x</button>
              </span>
            </div>
            <textarea
              ref="assistantInputRef"
              v-model="aiInput"
              rows="3"
              :placeholder="assistantPlaceholder"
              @keydown="onAIKeydown"
            ></textarea>
            <div class="assistant-composer-tools">
              <div class="assistant-left-tools">
                <div class="assistant-attach-wrap">
                  <button class="assistant-tool-btn" aria-label="添加文件或笔记" title="添加文件或笔记" @click="toggleAssistantAttachmentMenu">+</button>
                  <div v-if="assistantAttachmentMenuOpen" class="assistant-attach-menu">
                    <button type="button" @click="openAssistantNotePicker">选择笔记</button>
                    <button type="button" @click="openAssistantFilePicker">选择本地文件</button>
                  </div>
                </div>
                <button
                  class="assistant-tool-btn"
                  :class="{ active: assistantScope === 'library' }"
                  title="全库问答"
                  @click="runAssistantAction('library')"
                >Q</button>
              </div>
              <div class="assistant-right-tools">
                <span>{{ assistantScopeText }}</span>
                <button class="assistant-send" :disabled="aiLoading || assistantAttachmentReading || (!aiInput.trim() && !assistantAttachments.length)" @click="askAI">
                  {{ aiLoading ? "..." : "↑" }}
                </button>
              </div>
            </div>
            <div v-if="assistantNotePickerOpen" class="assistant-note-picker">
              <div class="assistant-note-picker-head">
                <strong>选择笔记</strong>
                <button class="ghost-btn" @click="assistantNotePickerOpen = false">完成</button>
              </div>
              <input v-model="assistantNoteSearch" type="search" placeholder="搜索笔记..." />
              <div class="assistant-note-list">
                <button
                  v-for="note in filteredAssistantNotes"
                  :key="`assistant-note-${note.id}`"
                  type="button"
                  @click="addAssistantNoteAttachment(note)"
                >
                  <span>{{ note.title || "未命名" }}</span>
                  <small>{{ noteFullPath(note) }}</small>
                </button>
                <p v-if="!filteredAssistantNotes.length" class="muted">没有可选笔记。</p>
              </div>
            </div>
          </div>

          <div v-if="!aiMessages.length" class="assistant-quick-start">
            <div class="assistant-text-columns">
              <div class="assistant-text-group">
                <span>最近对话</span>
                <button
                  v-for="item in assistantHistoryItems.slice(0, 2)"
                  :key="`recent-assistant-${item.key}`"
                  class="assistant-text-action"
                  @click="selectAssistantHistory(item.key)"
                >
                  <em>○</em>
                  <strong>{{ item.title }}</strong>
                </button>
                <p v-if="!assistantHistoryItems.length" class="muted">暂无对话</p>
              </div>
              <div class="assistant-text-group">
                <span>建议</span>
                <button class="assistant-text-action" @click="runAssistantAction('today')">
                  <em>P</em>
                  <strong>整理今日计划</strong>
                </button>
                <button class="assistant-text-action" @click="runAssistantAction('library')">
                  <em>Q</em>
                  <strong>全库问答</strong>
                </button>
                <button class="assistant-text-action" @click="runAssistantAction('recommend')">
                  <em>R</em>
                  <strong>内容推荐</strong>
                </button>
                <button class="assistant-text-action" @click="runAssistantAction('weekly')">
                  <em>W</em>
                  <strong>生成周报</strong>
                </button>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section v-else-if="activeView === 'workspace'" class="smart-workspace-view">
        <section class="smart-hero">
          <div class="smart-hero-main">
            <div class="smart-eyebrow">Smart workspace</div>
            <h2>智能知识工作台</h2>
            <p>这里汇总笔记状态、知识关系和主题研究，让学习资料从记录进入整理、复习和产出。</p>
          </div>
          <div class="smart-stat-grid">
            <div v-for="stat in workspaceStats" :key="stat.label" class="smart-stat">
              <strong>{{ stat.value }}</strong>
              <span>{{ stat.label }}</span>
            </div>
          </div>
        </section>

        <section class="tool-nav-grid">
          <button class="tool-nav-card cards" @click="openToolView('cards')">
            <small>Knowledge cards</small>
            <strong>知识卡片</strong>
            <span>进入独立卡片页面，创建正反面卡片并按艾宾浩斯曲线复习。</span>
          </button>
          <button class="tool-nav-card graph" @click="openToolView('knowledgeGraph')">
            <small>Knowledge graph</small>
            <strong>知识地图</strong>
            <span>查看笔记之间的链接、共享标签和相似内容关系。</span>
          </button>
          <button class="tool-nav-card research" @click="openToolView('researchStudio')">
            <small>Research studio</small>
            <strong>主题研究室</strong>
            <span>输入主题，生成提纲、知识缺口、后续问题和卡片。</span>
          </button>
          <button class="tool-nav-card tasks" @click="openToolView('tasks')">
            <small>Todo list</small>
            <strong>任务中心</strong>
            <span>进入完整计划页面，管理日期、优先级和完成状态。</span>
          </button>
          <button class="tool-nav-card templates" @click="openToolView('templates')">
            <small>Template library</small>
            <strong>模板库</strong>
            <span>新增、编辑、删除模板，并用已有标签多选。</span>
          </button>
          <button class="tool-nav-card import" @click="openToolView('import')">
            <small>Import</small>
            <strong>文件导入</strong>
            <span>从电脑选择文件，并指定目标文件夹。</span>
          </button>
          <button class="tool-nav-card recommend" @click="openToolView('recommend')">
            <small>Recommend</small>
            <strong>推荐与回顾</strong>
            <span>输入主题并选择参考笔记，让 AI 生成推荐与总结。</span>
          </button>
          <button class="tool-nav-card quality-hub" @click="openToolView('qualityHub')">
            <small>Quality hub</small>
            <strong>知识体检中心</strong>
            <span>扫描缺标签、短内容、根目录散落和结构不足的笔记。</span>
          </button>
          <button class="tool-nav-card writing" @click="openToolView('writingStudio')">
            <small>Writing studio</small>
            <strong>写作中心</strong>
            <span>调用 AI 自动整理本周学习周报，并创建到指定文件夹。</span>
          </button>
        </section>

</section>

      <section v-else-if="activeView === 'cards'" class="tool-page cards-page">
        <div class="tool-page-head">
          <div>
            <small>Knowledge cards</small>
            <h1>知识卡片</h1>
            <p>管理你的知识卡片。复习会进入独立页面，一次只看一张卡片。</p>
          </div>
          <div class="panel-tools">
            <button class="btn" @click="openWorkspace">返回工作台</button>
            <button class="btn" :disabled="!dueKnowledgeCards.length" @click="openToolView('cardReview')">开始复习</button>
            <button class="btn primary" @click="openCardForm()">新建卡片</button>
          </div>
        </div>
        <section class="card-dashboard">
          <div class="smart-stat-grid">
            <div v-for="stat in cardStats" :key="`card-stat-${stat.label}`" class="smart-stat">
              <strong>{{ stat.value }}</strong>
              <span>{{ stat.label }}</span>
            </div>
          </div>
          <div class="dashboard-charts card-only-charts">
            <div class="chart-panel">
              <div class="panel-head"><strong>卡片状态</strong><span>饼图</span></div>
              <div class="pie-chart-wrap">
                <svg viewBox="0 0 42 42" class="pie-chart">
                  <circle cx="21" cy="21" r="15.9" fill="transparent" stroke="#edf0f4" stroke-width="6"></circle>
                  <circle
                    v-for="(item, index) in cardStatusPie"
                    :key="`card-page-pie-${item.label}`"
                    cx="21"
                    cy="21"
                    r="15.9"
                    fill="transparent"
                    :stroke="['#2563eb', '#7c3aed', '#64748b'][index] || '#111827'"
                    stroke-width="6"
                    :stroke-dasharray="pieDashArray(item, cardStatusPie)"
                    :stroke-dashoffset="pieDashOffset(index, cardStatusPie)"
                  ></circle>
                </svg>
                <div class="chart-legend">
                  <span v-for="item in cardStatusPie" :key="`card-page-legend-${item.label}`">{{ item.label }} {{ item.value }}</span>
                </div>
              </div>
            </div>
            <div class="chart-panel wide">
              <div class="panel-head"><strong>复习队列</strong><span>柱状图</span></div>
              <div class="bar-chart">
                <div v-for="item in cardReviewBars" :key="`card-bar-${item.label}`" class="bar-item">
                  <span>{{ item.label }}</span>
                  <div><em :style="{ width: `${Math.max(4, (Number(item.value || 0) / chartMax(cardReviewBars)) * 100)}%` }"></em></div>
                  <strong>{{ item.value }}</strong>
                </div>
              </div>
            </div>
            <div class="chart-panel full">
              <div class="panel-head"><strong>14 天复习趋势</strong><span>折线图</span></div>
              <svg viewBox="0 0 300 130" class="line-chart">
                <polyline :points="lineChartPoints(cardReviewTrend)" fill="none" stroke="#2563eb" stroke-width="3" />
              </svg>
              <div class="chart-legend horizontal"><span>蓝色：完成复习</span></div>
            </div>
          </div>
        </section>
        <section class="cards-layout single">
          <div class="card-library page-panel">
            <div class="card-toolbar">
              <input v-model="cardSearch" type="search" placeholder="搜索问题、答案或标签" />
              <select v-model="cardFilter">
                <option value="active">复习中</option>
                <option value="mastered">已掌握</option>
                <option value="archived">已归档</option>
                <option value="all">全部</option>
              </select>
            </div>
            <div class="knowledge-card-list">
              <div v-for="card in filteredCards" :key="`card-${card.id}`" class="knowledge-card">
                <div>
                  <small>{{ cardStatusLabel(card.status) }} · 阶段 {{ card.review_stage || 0 }}</small>
                  <strong>{{ card.front }}</strong>
                  <p>{{ card.back }}</p>
                  <div v-if="card.tags?.length" class="reader-tags">
                    <span v-for="tag in card.tags" :key="`card-tag-${card.id}-${tag}`" class="tag-chip static">{{ tag }}</span>
                  </div>
                </div>
                <div class="card-actions">
                  <span>{{ formatDateTime(card.next_review_at) }}</span>
                  <button class="ghost-btn compact" @click="openCardForm(card)">编辑</button>
                  <button v-if="card.status === 'mastered'" class="ghost-btn compact" @click="restartKnowledgeCardReview(card)">重新复习</button>
                  <button class="ghost-btn compact" @click="archiveKnowledgeCard(card)">{{ card.status === "archived" ? "恢复" : "归档" }}</button>
                  <button class="ghost-btn compact danger-text" @click="deleteKnowledgeCard(card)">删除</button>
                </div>
              </div>
              <p v-if="!filteredCards.length" class="muted">没有匹配的知识卡片。</p>
            </div>
          </div>
        </section>
        <div v-if="cardFormOpen" class="modal-mask" @click.self="closeCardForm">
          <section class="template-modal card-modal">
            <div class="modal-head">
              <strong>{{ editingCardID ? "编辑知识卡片" : "新建知识卡片" }}</strong>
              <button class="ghost-btn" @click="closeCardForm">关闭</button>
            </div>
            <input v-model="cardForm.front" type="text" placeholder="正面：写问题" />
            <textarea v-model="cardForm.back" rows="6" placeholder="背面：写答案"></textarea>
            <div class="form-actions">
              <button class="btn primary" :disabled="cardSaving || !cardForm.front.trim() || !cardForm.back.trim()" @click="saveKnowledgeCard">
                {{ cardSaving ? "保存中" : "保存卡片" }}
              </button>
            </div>
          </section>
        </div>
      </section>

      <section v-else-if="activeView === 'cardReview'" class="card-review-page">
        <div class="card-review-top">
          <button class="btn" @click="openToolView('cards')">返回卡片库</button>
          <div>
            <strong>知识卡片复习</strong>
            <span>已完成 {{ reviewSessionDone }} 张 · 剩余 {{ reviewSessionCards.length }} 张</span>
          </div>
        </div>
        <section v-if="currentReviewCard" class="review-stage">
          <button class="review-big-card" :class="{ flipped: cardAnswerVisible }" @click="cardAnswerVisible = true">
            <small>{{ cardAnswerVisible ? "答案" : "问题" }}</small>
            <strong>{{ cardAnswerVisible ? currentReviewCard.back : currentReviewCard.front }}</strong>
          </button>
          <div class="review-stage-actions">
            <template v-if="!cardAnswerVisible">
              <button class="btn primary" @click="cardAnswerVisible = true">显示答案</button>
            </template>
            <template v-else>
              <button class="btn review-forgot" :disabled="cardReviewing" @click="reviewCurrentCard(false)">不记得</button>
              <button class="btn primary review-remember" :disabled="cardReviewing" @click="reviewCurrentCard(true)">记得</button>
            </template>
          </div>
          <p class="muted">当前阶段 {{ currentReviewCard.review_stage || 0 }}。选择“记得”进入下一个周期；选择“不记得”留在当前周期。</p>
        </section>
        <section v-else class="review-finished page-panel">
          <strong>本轮复习完成</strong>
          <p>没有更多到期卡片了。忘记的卡片仍留在当前周期，下次进入复习会继续出现。</p>
          <div class="review-stage-actions">
            <button class="btn" @click="openToolView('cards')">返回卡片库</button>
            <button class="btn primary" @click="startCardReviewSession">再检查一遍</button>
          </div>
        </section>
      </section>

      <section v-else-if="activeView === 'knowledgeGraph'" class="tool-page graph-page">
        <div class="tool-page-head">
          <div>
            <small>Knowledge graph</small>
            <h1>知识地图</h1>
            <p>只展示笔记关系，连线来自内部链接、共享标签和内容相似度。</p>
          </div>
          <div class="panel-tools">
            <button class="btn" @click="openWorkspace">返回工作台</button>
            <button class="btn" :disabled="graphLoading" @click="loadWorkspaceGraph">{{ graphLoading ? "刷新中" : "刷新" }}</button>
          </div>
        </div>
        <section class="graph-tools page-panel">
          <input v-model="graphQuery" type="search" placeholder="搜索节点或路径" @input="clearGraphFocus" />
          <select v-model="graphRelationFilter" @change="clearGraphFocus">
            <option value="important">精选关系</option>
            <option value="all">全部关系</option>
            <option value="link">内部链接</option>
            <option value="tag">共享标签</option>
            <option value="similar">内容相似</option>
          </select>
          <select v-model="graphTagFilter" @change="clearGraphFocus">
            <option value="">全部标签</option>
            <option v-for="tag in graphTags" :key="`graph-tag-${tag}`" :value="tag">{{ tag }}</option>
          </select>
          <button v-if="graphTagFilter || graphQuery || graphFocusID" class="ghost-btn compact" @click="graphTagFilter = ''; graphQuery = ''; clearGraphFocus()">重置视图</button>
          <span>{{ graphVisibleNodes.length }} 个节点 / 显示 {{ graphVisibleEdges.length }} 条关系<span v-if="graphVisibleEdges.length < filteredGraphEdges.length">（共 {{ filteredGraphEdges.length }} 条）</span></span>
          <div class="graph-legend" aria-label="关系图例">
            <template v-if="graphSearchQuery">
              <span><i class="link outgoing"></i>节点链接出去</span>
              <span><i class="incoming"></i>链接到它</span>
            </template>
            <template v-else>
              <span><i class="link"></i>内部链接</span>
              <span><i class="tag"></i>共享标签</span>
              <span><i class="similar"></i>内容相似</span>
            </template>
          </div>
        </section>
        <section class="graph-board">
          <aside class="graph-sidebar page-panel">
            <div v-if="graphFocusedNode" class="graph-focus-card">
              <small>当前邻域</small>
              <strong>{{ graphFocusedNode.title }}</strong>
              <span>{{ graphFocusedNode.path }}</span>
              <div>
                <button class="ghost-btn compact" @click="openNote(graphFocusedNode.id)">打开笔记</button>
                <button class="ghost-btn compact" @click="clearGraphFocus">回到概览</button>
              </div>
            </div>
            <div class="graph-side-section">
              <strong>主题分组</strong>
              <button
                v-for="topic in graphTopicSummary"
                :key="`graph-topic-${topic.label}`"
                type="button"
                class="graph-topic-row"
                :class="{ active: graphTagFilter === topic.label }"
                @click="selectGraphTopic(graphTagFilter === topic.label ? '' : topic.label)"
              >
                <i :style="{ background: topic.color }"></i>
                <span>{{ topic.label }}</span>
                <em>{{ topic.count }}</em>
              </button>
            </div>
            <div class="graph-side-section">
              <strong>高连接节点</strong>
              <button
                v-for="node in graphHubNodes"
                :key="`graph-hub-${node.id}`"
                type="button"
                class="graph-hub-row"
                :class="{ active: Number(graphFocusID) === Number(node.id) }"
                @click="focusGraphNode(node.id)"
              >
                <span>{{ node.title }}</span>
                <small>{{ Math.round(node.degree || 0) }} 关系强度</small>
              </button>
            </div>
          </aside>
          <section class="graph-canvas page-panel">
            <svg viewBox="0 0 1180 720" class="knowledge-graph-svg">
              <g class="graph-clusters">
                <g v-for="cluster in graphClusterLabels" :key="`cluster-${cluster.label}`">
                  <text :x="cluster.x" :y="cluster.y">{{ cluster.label }} · {{ cluster.count }}</text>
                </g>
              </g>
              <line
                v-for="edge in graphLayoutEdges"
                :key="`edge-${edge.source}-${edge.target}-${edge.type}`"
                :x1="edge.sourceNode.x"
                :y1="edge.sourceNode.y"
                :x2="edge.targetNode.x"
                :y2="edge.targetNode.y"
                :class="['graph-edge', edge.type, edge.searchDirection]"
                :stroke-width="0.7 + Number(edge.weight || 0) * 1.8"
              />
              <g
                v-for="node in graphLayoutNodes"
                :key="`node-${node.id}`"
                class="graph-node"
                :class="{ focused: Number(graphFocusID) === Number(node.id) }"
                @click="focusGraphNode(node.id)"
                @dblclick="openNote(node.id)"
              >
                <title>{{ node.path || node.title }}&#10;关系强度 {{ node.degree || 0 }} · 质量 {{ node.quality_score || 0 }}&#10;单击聚焦邻域，双击打开笔记</title>
                <circle :cx="node.x" :cy="node.y" :r="node.radius" :style="{ '--node-color': node.color }" />
                <g v-if="node.showLabel" class="graph-label">
                  <rect :x="node.labelX" :y="node.labelY" :width="node.labelWidth" height="26" rx="6" />
                  <text :x="node.labelX + 11" :y="node.labelY + 17">{{ node.label }}</text>
                </g>
              </g>
            </svg>
            <p v-if="!graphLayoutNodes.length" class="muted">还没有可展示的笔记节点。</p>
          </section>
        </section>
      </section>

      <section v-else-if="activeView === 'researchStudio'" class="tool-page research-page">
        <div class="tool-page-head">
          <div>
            <small>Research studio</small>
            <h1>主题研究室</h1>
            <p>输入研究主题，系统会结合已有笔记生成提纲、知识缺口和可沉淀为知识卡片的问题。</p>
          </div>
          <div class="panel-tools">
            <div class="research-history-menu">
              <button
                class="research-history-toggle"
                title="历史记录"
                aria-label="历史记录"
                :aria-expanded="researchHistoryOpen"
                @click="researchHistoryOpen = !researchHistoryOpen"
              >
                <span class="clock-icon"></span>
                <span v-if="researchHistoryItems.length" class="research-history-count">{{ researchHistoryItems.length }}</span>
              </button>
              <aside v-if="researchHistoryOpen" class="research-history-panel page-panel">
                <div class="panel-head"><strong>历史记录</strong><span>{{ researchHistoryItems.length }}</span></div>
                <button
                  v-for="item in researchHistoryItems"
                  :key="item.id"
                  class="research-history-item"
                  :class="{ active: researchResult?.topic === item.topic }"
                  @click="openResearchHistory(item)"
                >
                  <span>{{ item.topic }}</span>
                  <small>{{ formatDateTime(new Date(Number(item.createdAt || 0)).toISOString()) }}</small>
                  <em @click.stop="deleteResearchHistory(item)">删除</em>
                </button>
                <p v-if="!researchHistoryItems.length" class="muted">还没有历史记录。生成一次研究方案后会自动保存。</p>
              </aside>
            </div>
            <button class="btn" @click="openWorkspace">返回工作台</button>
          </div>
        </div>
        <section class="research-composer page-panel">
          <input v-model="researchTopic" type="search" placeholder="输入研究主题，例如：本地 RAG 笔记系统、Go 后端设计" @input="onResearchTopicInput" @keydown.enter="runResearchSession" />
          <button class="btn primary" :disabled="researchLoading || !researchTopic.trim()" @click="runResearchSession">
            {{ researchLoading ? "研究中..." : "生成研究方案" }}
          </button>
        </section>
        <section class="research-layout">
          <section v-if="researchResult" class="research-result-grid">
            <div class="page-panel research-summary">
              <div class="panel-head"><strong>研究摘要</strong><span>{{ researchResult.used_ai ? "AI" : "本地规则" }}</span></div>
              <p>{{ researchResult.summary }}</p>
            </div>
            <div class="page-panel">
              <div class="panel-head"><strong>相关笔记</strong></div>
              <button v-for="note in researchResult.related_notes || []" :key="`research-note-${note.id}`" class="smart-list-item" @click="openNote(note.id)">
                <span>{{ refTitle(note) }}</span>
                <small>{{ note.path }}</small>
              </button>
            </div>
            <div class="page-panel">
              <div class="panel-head"><strong>研究提纲</strong></div>
              <ol class="outline-list">
                <li v-for="item in researchResult.outline || []" :key="`research-outline-${item}`">{{ item }}</li>
              </ol>
            </div>
            <div class="page-panel">
              <div class="panel-head"><strong>知识缺口</strong></div>
              <div v-for="item in researchResult.gaps || []" :key="`research-gap-${item}`" class="quality-issue">
                <strong>{{ item }}</strong>
                <span>可以补充到笔记或卡片中。</span>
              </div>
            </div>
            <div class="page-panel full">
              <div class="panel-head"><strong>后续问题</strong><span>可一键转卡片</span></div>
              <div class="research-question-list">
                <div v-for="q in researchResult.questions || []" :key="`research-question-${q}`" class="research-question">
                  <span>{{ q }}</span>
                  <button class="ghost-btn compact" :disabled="researchCardCreating === q" @click="createCardFromResearchQuestion(q)">
                    {{ researchCardCreating === q ? "创建中" : "转为卡片" }}
                  </button>
                </div>
              </div>
            </div>
            <div class="page-panel full">
              <div class="panel-head"><strong>建议新建笔记</strong></div>
              <div class="suggest-note-list">
                <span v-for="item in researchResult.suggested_notes || []" :key="`suggest-note-${item}`">{{ item }}</span>
              </div>
            </div>
          </section>
          <section v-else class="research-empty page-panel">
            <strong>暂无研究结果</strong>
            <p>输入主题并生成后，这里会显示结果，同时自动保存到左侧历史记录。</p>
          </section>
        </section>
      </section>

      <section v-else-if="activeView === 'calendar'" class="calendar-page">
        <aside class="mini-calendar page-panel">
          <div class="calendar-mini-head">
            <strong>{{ calendarMonthLabel }}</strong>
            <button class="ghost-btn" @click="selectCalendarDay(localDateKey(new Date()))">今天</button>
          </div>
          <div class="mini-weekdays">
            <span>日</span><span>一</span><span>二</span><span>三</span><span>四</span><span>五</span><span>六</span>
          </div>
          <div class="mini-days">
            <button
              v-for="day in calendarMonthDays"
              :key="day.key"
              class="mini-day"
              :class="{ muted: !day.currentMonth, today: day.today, selected: day.selected, busy: day.tasks.length }"
              @click="selectCalendarDay(day.key)"
            >
              <span>{{ day.day }}</span>
            </button>
          </div>
        </aside>

        <section class="calendar-detail page-panel">
          <div class="calendar-detail-head">
            <div>
              <small>Plan calendar</small>
              <strong>{{ calendarSelectedDateText }}</strong>
            </div>
            <div class="calendar-actions">
              <button class="btn" :title="`快捷键：${calendarShortcutLabel}`" @click="openCalendarCommand">命令</button>
              <button class="btn primary" @click="createPlanOnDate(calendarSelectedDate)">创建plan</button>
            </div>
          </div>
          <div class="calendar-week">
            <div class="calendar-time-col">
              <span>GMT+8</span>
              <span v-for="hour in calendarHours" :key="`h-${hour}`">{{ hour }}:00</span>
            </div>
            <div
              v-for="day in calendarWeekDays"
              :key="`week-${day.key}`"
              class="calendar-day-col"
              :class="{ today: day.today, selected: day.selected }"
            >
              <button class="calendar-day-head" @click="selectCalendarDay(day.key)">
                <span>{{ day.label }}</span>
                <strong>{{ day.day }}</strong>
              </button>
              <div class="calendar-day-body">
                <div
                  v-for="hour in calendarHours"
                  :key="`slot-${day.key}-${hour}`"
                  class="calendar-hour-slot"
                  @contextmenu.prevent="openCalendarContextMenu($event, day.key, hour)"
                >
                  <button
                    v-for="task in tasksForDateHour(day.key, hour)"
                    :key="`cal-task-${task.id}`"
                    class="calendar-plan"
                    :class="[`priority-${task.priority || 'medium'}`, { done: task.done, active: selectedCalendarTask?.id === task.id }]"
                    @click.stop="selectCalendarTask(task)"
                  >
                    <span>{{ formatTaskTime(task) }}</span>
                    <strong>{{ task.title }}</strong>
                  </button>
                </div>
              </div>
            </div>
          </div>
        </section>

        <aside class="calendar-inspector page-panel">
          <div class="calendar-inspector-head">
            <div>
              <small>Plan</small>
              <strong>{{ selectedCalendarTask ? "详细信息" : "当天 plan" }}</strong>
            </div>
            <button class="ghost-btn" @click="createPlanOnDate(calendarSelectedDate)">新建</button>
          </div>
          <template v-if="selectedCalendarTask">
            <label>
              <span>标题</span>
              <input
                :value="selectedCalendarTask.title"
                type="text"
                @input="updatePlanTaskField(selectedCalendarTask, 'title', $event.target.value)"
              />
            </label>
            <div class="calendar-time-editor">
              <label>
                <span>日期</span>
                <input
                  :value="selectedCalendarTask.due"
                  type="date"
                  @input="updatePlanTaskField(selectedCalendarTask, 'due', $event.target.value)"
                />
              </label>
              <label>
                <span>开始</span>
                <input
                  :value="selectedCalendarTask.start_time"
                  type="time"
                  @input="updatePlanTaskField(selectedCalendarTask, 'start_time', $event.target.value)"
                />
              </label>
              <label>
                <span>结束</span>
                <input
                  :value="selectedCalendarTask.end_time"
                  type="time"
                  @input="updatePlanTaskField(selectedCalendarTask, 'end_time', $event.target.value)"
                />
              </label>
            </div>
            <label>
              <span>紧急情况</span>
              <select
                :value="selectedCalendarTask.priority"
                @change="updatePlanTaskField(selectedCalendarTask, 'priority', $event.target.value)"
              >
                <option value="high">紧急</option>
                <option value="medium">普通</option>
                <option value="low">不紧急</option>
              </select>
            </label>
            <label>
              <span>描述</span>
              <textarea
                :value="selectedCalendarTask.description"
                placeholder="补充这个 plan 的上下文、目标或注意事项。"
                @input="updatePlanTaskField(selectedCalendarTask, 'description', $event.target.value)"
              ></textarea>
            </label>
            <div class="calendar-inspector-actions">
              <button class="btn" @click="togglePlanTask(selectedCalendarTask)">
                {{ selectedCalendarTask.done ? "标记未完成" : "标记完成" }}
              </button>
              <button class="btn danger" @click="deletePlanTask(selectedCalendarTask)">删除</button>
            </div>
          </template>
          <template v-else>
            <button
              v-for="task in calendarSelectedDateTasks"
              :key="`selected-cal-${task.id}`"
              class="tool-row-card compact"
              @click="selectCalendarTask(task)"
            >
              <span>{{ task.title }}</span>
              <small>{{ formatTaskTime(task) }} · {{ priorityLabel(task.priority) }}{{ task.done ? " · Done" : "" }}</small>
            </button>
            <p v-if="!calendarSelectedDateTasks.length" class="muted">这一天还没有 plan。可以在周视图里右键创建。</p>
          </template>
        </aside>
      </section>

      <section v-else-if="activeView === 'tasks'" class="tool-page">
        <div class="tool-page-head">
          <div>
            <small>Todo list</small>
            <h1>任务中心</h1>
            <p>一个独立的计划页面，用来安排自己的任务；这里创建的 plan 会同步显示在日历中。</p>
          </div>
          <div class="panel-tools">
            <button class="btn" @click="openWorkspace">返回工作台</button>
            <button class="info-btn" @click="openGuide('tasks')">!</button>
          </div>
        </div>
        <div class="todo-page-layout single">
          <section class="todo-panel page-panel">
            <div class="todo-overview">
              <div>
                <small>Open</small>
                <strong>{{ pendingTasks.length }}</strong>
              </div>
              <div>
                <small>Today</small>
                <strong>{{ todayPlanTasks.length }}</strong>
              </div>
              <div>
                <small>High</small>
                <strong>{{ highPlanTasks.length }}</strong>
              </div>
              <div>
                <small>Done</small>
                <strong>{{ completedTasks.length }}</strong>
              </div>
            </div>
            <div class="todo-progress">
              <div>
                <strong>{{ planProgress }}%</strong>
                <span>completed</span>
              </div>
              <div class="todo-progress-bar"><span :style="{ width: `${planProgress}%` }"></span></div>
            </div>
            <div class="todo-composer">
              <input v-model="planTaskTitle" type="text" placeholder="Add a plan..." @keydown.enter="addPlanTask" />
              <input v-model="planTaskDate" type="date" />
              <input v-model="planTaskStartTime" type="time" aria-label="开始时间" />
              <input v-model="planTaskEndTime" type="time" aria-label="结束时间" />
              <select v-model="planTaskPriority">
                <option value="high">High</option>
                <option value="medium">Medium</option>
                <option value="low">Low</option>
              </select>
              <button class="btn primary" @click="addPlanTask">Add</button>
            </div>
            <div class="todo-tabs">
              <button :class="{ active: planTaskFilter === 'open' }" @click="planTaskFilter = 'open'">Open</button>
              <button :class="{ active: planTaskFilter === 'today' }" @click="planTaskFilter = 'today'">Today</button>
              <button :class="{ active: planTaskFilter === 'done' }" @click="planTaskFilter = 'done'">Done</button>
              <button :class="{ active: planTaskFilter === 'all' }" @click="planTaskFilter = 'all'">All</button>
            </div>
            <div class="todo-list">
              <div v-for="task in filteredPlanTasks" :key="`page-task-${task.id}`" class="todo-item" :class="[{ done: task.done }, `priority-${task.priority || 'medium'}`]">
                <button class="todo-check" @click="togglePlanTask(task)">{{ task.done ? "✓" : "" }}</button>
                <div class="todo-main">
                  <strong>{{ task.title }}</strong>
                  <small><span :class="`priority-dot ${task.priority}`"></span>{{ priorityLabel(task.priority) }}<template v-if="task.due"> · {{ task.due }}</template> · {{ formatTaskTime(task) }}</small>
                </div>
                <button class="todo-delete" @click="deletePlanTask(task)">×</button>
              </div>
              <p v-if="!filteredPlanTasks.length" class="muted">没有匹配的计划。</p>
            </div>
          </section>
        </div>
      </section>

      <section v-else-if="activeView === 'templates'" class="tool-page">
        <div class="tool-page-head">
          <div>
            <small>Template library</small>
            <h1>模板库</h1>
            <p>维护你的笔记模板。标签从已有标签中多选，也可以在下拉框中新建。</p>
          </div>
          <div class="panel-tools">
            <button class="btn" @click="openWorkspace">返回工作台</button>
            <button class="btn primary" @click="openTemplateModal()">新增模板</button>
            <button class="info-btn" @click="openGuide('templates')">!</button>
          </div>
        </div>
        <div class="template-grid template-page-grid">
          <div v-for="tpl in workspaceTemplates" :key="`page-template-${tpl.key}`" class="template-tile">
            <button class="template-create" @click="createTemplateNote(tpl.key)">
              <strong>{{ tpl.name }}</strong>
            </button>
            <small>{{ (tpl.tags || []).join(" / ") }}</small>
            <pre>{{ tpl.markdown }}</pre>
            <div class="template-actions">
              <button @click="openTemplateModal(tpl)">编辑</button>
              <button @click="deleteTemplate(tpl)">删除</button>
            </div>
          </div>
        </div>
      </section>

      <section v-else-if="activeView === 'import'" class="tool-page">
        <div class="tool-page-head">
          <div>
            <small>Import</small>
            <h1>文件导入</h1>
            <p>从电脑选择文本类文件，指定目标文件夹后创建笔记。</p>
          </div>
          <div class="panel-tools">
            <button class="btn" @click="openWorkspace">返回工作台</button>
            <button class="info-btn" @click="openGuide('import')">!</button>
          </div>
        </div>
        <section class="import-modal inline-tool">
          <label class="file-picker">
            <input type="file" accept=".md,.markdown,.txt,.csv,.json,.log" @change="onImportFileChange" />
            <span>{{ importReading ? "正在读取..." : (importFileName || "选择本地文件") }}</span>
          </label>
          <input v-model="importTitle" type="text" placeholder="标题，默认使用文件名" />
          <div class="folder-select-row">
            <select v-model="importParentID">
              <option value="">根目录</option>
              <option v-for="folder in folderOptions" :key="`import-page-folder-${folder.id}`" :value="folder.id">{{ folder.title }}</option>
            </select>
            <button class="btn" type="button" @click="createFolderForTarget('import')">新建文件夹</button>
          </div>
          <div class="template-tag-picker import-tag-picker">
            <button class="tag-trigger" type="button" @click="importTagMenuOpen = !importTagMenuOpen">
              <span v-if="!(importTags || []).length" class="tag-placeholder">选择或新建标签</span>
              <span v-else class="tag-chip-list">
                <span v-for="tag in importTags" :key="`import-tag-${tag}`" class="tag-chip">
                  {{ tag }}
                  <span class="tag-chip-close" @click.stop="removeImportTag(tag)">x</span>
                </span>
              </span>
              <span class="tag-caret" :class="{ open: importTagMenuOpen }">v</span>
            </button>
            <div v-if="importTagMenuOpen" class="tag-menu template-tag-menu">
              <input v-model="importTagQuery" type="text" placeholder="搜索或新增标签" @keydown.enter.prevent="createImportTagFromQuery" />
              <button v-if="canCreateImportTag" class="tag-option create" type="button" @click="createImportTagFromQuery">
                新增标签 {{ importTagQuery }}
              </button>
              <div class="tag-options">
                <button
                  v-for="tag in importTagOptions"
                  :key="`import-opt-${tag}`"
                  class="tag-option"
                  type="button"
                  @click="toggleImportTag(tag)"
                >
                  <span>{{ tag }}</span>
                  <span class="check">{{ importTagSet.has(String(tag).toLowerCase()) ? "v" : "" }}</span>
                </button>
              </div>
            </div>
          </div>
          <textarea v-model="importText" readonly placeholder="选择文件后会在这里预览文本内容"></textarea>
          <div class="import-actions">
            <button class="btn primary" :disabled="!importText.trim() || importReading" @click="submitImportNote">导入笔记系统</button>
          </div>
        </section>
      </section>

      <section v-else-if="activeView === 'recommend'" class="tool-page">
        <div class="tool-page-head">
          <div>
            <small>Recommend</small>
            <h1>外部资源推荐</h1>
            <p>输入主题，系统会检索互联网文章、视频和论文链接；选择笔记只作为理解上下文，不作为推荐结果。</p>
          </div>
          <div class="panel-tools">
            <div class="research-history-menu recommend-history-menu">
              <button
                class="research-history-toggle"
                title="历史记录"
                aria-label="历史记录"
                :aria-expanded="recommendHistoryOpen"
                @click="recommendHistoryOpen = !recommendHistoryOpen"
              >
                <span class="clock-icon"></span>
                <span v-if="recommendHistoryItems.length" class="research-history-count">{{ recommendHistoryItems.length }}</span>
              </button>
              <aside v-if="recommendHistoryOpen" class="research-history-panel page-panel">
                <div class="panel-head"><strong>历史记录</strong><span>{{ recommendHistoryItems.length }}</span></div>
                <button
                  v-for="item in recommendHistoryItems"
                  :key="item.id"
                  class="research-history-item"
                  :class="{ active: recommendResult?.topic === item.topic }"
                  @click="openRecommendHistory(item)"
                >
                  <span>{{ item.topic }}</span>
                  <small>{{ formatDateTime(new Date(Number(item.createdAt || 0)).toISOString()) }}</small>
                  <em @click.stop="deleteRecommendHistory(item)">删除</em>
                </button>
                <p v-if="!recommendHistoryItems.length" class="muted">还没有历史记录。生成一次推荐后会自动保存。</p>
              </aside>
            </div>
            <button class="btn" @click="openWorkspace">返回工作台</button>
            <button class="info-btn" @click="openGuide('recommend')">!</button>
          </div>
        </div>
        <section class="recommend-workbench">
          <div class="recommend-search page-panel">
            <div class="recommend-input-row">
              <input
                v-model="recommendQuery"
                type="search"
                placeholder="输入希望被推荐的主题，例如：Go 并发项目实践、RAG 产品设计、前端性能优化"
                @input="onRecommendTopicInput"
                @keydown.enter="runAIRecommendation"
              />
              <button class="icon-btn note-picker-btn" title="选择参考上下文" @click="recommendNotePickerOpen = !recommendNotePickerOpen">
                <span class="file-action-icon"></span>
              </button>
              <button class="btn primary" :disabled="recommendLoading || (!recommendQuery.trim() && !recommendSelectedIDs.length)" @click="runAIRecommendation">
                {{ recommendLoading ? "生成中..." : "生成推荐" }}
              </button>
            </div>
            <div v-if="recommendSelectedNotes.length" class="selected-note-row">
              <span v-for="note in recommendSelectedNotes" :key="`rec-selected-${note.id}`" class="tag-chip">
                {{ note.title || "未命名" }}
                <span class="tag-chip-close" @click="toggleRecommendNote(note)">x</span>
              </span>
              <button class="ghost-btn" @click="clearRecommendNotes">清空</button>
            </div>
            <div v-if="recommendNotePickerOpen" class="recommend-note-picker">
              <div class="search-tag-dropdown-head">
                <strong>选择参考上下文</strong>
                <button class="ghost-btn" @click="recommendNotePickerOpen = false">完成</button>
              </div>
              <div class="recommend-note-filter">
                <input
                  v-model="recommendNoteSearch"
                  type="search"
                  placeholder="搜索笔记标题、路径或标签"
                />
              </div>
              <div class="recommend-note-list">
                <button
                  v-for="note in filteredRecommendNotes"
                  :key="`rec-note-${note.id}`"
                  class="recommend-note-option"
                  :class="{ active: recommendSelectedIDs.some((id) => Number(id) === Number(note.id)) }"
                  @click="toggleRecommendNote(note)"
                >
                  <span class="row-icon note"></span>
                  <span>{{ noteFullPath(note) || note.title || "未命名" }}</span>
                  <small>{{ recommendSelectedIDs.some((id) => Number(id) === Number(note.id)) ? "已选" : "" }}</small>
                </button>
                <p v-if="!filteredRecommendNotes.length" class="muted">没有匹配的笔记。</p>
              </div>
            </div>
          </div>

          <div v-if="recommendResult" class="recommend-ai-result page-panel">
            <div class="panel-head">
              <div>
                <small>AI summary</small>
                <strong>外部资源总结</strong>
              </div>
              <span>{{ recommendResult.used_ai ? "AI" : "本地规则" }}</span>
            </div>
            <div v-if="recommendResult.resources?.length" class="external-resource-list">
              <a
                v-for="resource in recommendResult.resources"
                :key="`resource-${resource.url}`"
                class="external-resource-card"
                :href="resource.url"
                target="_blank"
                rel="noreferrer"
              >
                <span>{{ resource.kind || "资源" }}</span>
                <strong>{{ resource.title }}</strong>
                <small>{{ resource.source }} · {{ resource.url }}</small>
                <em v-if="resource.description">{{ resource.description }}</em>
              </a>
            </div>
            <pre>{{ recommendResult.summary }}</pre>
          </div>
        </section>
      </section>

      <section v-else-if="activeView === 'qualityHub'" class="tool-page">
        <div class="tool-page-head">
          <div>
            <small>Quality hub</small>
            <h1>知识体检中心</h1>
            <p>全库扫描笔记状态，集中查看还没有标记完成的内容。</p>
          </div>
          <div class="panel-tools">
            <button class="btn" @click="openWorkspace">返回工作台</button>
            <button class="info-btn" @click="openGuide('qualityHub')">!</button>
          </div>
        </div>
        <section class="big-tool-panel">
          <div class="tool-metric-grid">
            <div v-for="item in qualityHubStats" :key="item.label" class="tool-metric">
              <strong>{{ item.value }}</strong>
              <span>{{ item.label }}</span>
            </div>
          </div>
          <div class="tool-list-panel unfinished-in-quality">
            <div class="panel-head"><strong>未完成笔记</strong><span>{{ qualityUnfinishedNotes.length }}</span></div>
            <button
              v-for="item in qualityUnfinishedNotes"
              :key="`quality-unfinished-${item.id}`"
              class="tool-row-card compact"
              @click="openNote(item.id)"
            >
              <span>{{ refTitle(item) }}</span>
              <small>{{ item.path || noteFullPath(item) }}</small>
            </button>
            <p v-if="!qualityUnfinishedNotes.length" class="muted">笔记状态都已经完成。</p>
          </div>
        </section>
      </section>

      <section v-else-if="activeView === 'writingStudio'" class="tool-page">
        <div class="tool-page-head">
          <div>
            <small>Writing studio</small>
            <h1>写作中心</h1>
            <p>调用后端 AI 根据本周笔记生成学习周报，并创建到你选择的文件夹。</p>
          </div>
          <div class="panel-tools">
            <button class="btn" @click="openWorkspace">返回工作台</button>
            <button class="info-btn" @click="openGuide('writingStudio')">!</button>
          </div>
        </div>
        <section class="weekly-report-panel page-panel">
          <div class="panel-head">
            <div>
              <small>Weekly report</small>
              <strong>生成本周学习周报</strong>
            </div>
            <span>包含学习大纲、下周建议和资源推荐</span>
          </div>
          <div class="weekly-report-form">
            <input v-model="weeklyReportTitle" type="text" placeholder="周报标题" />
            <div class="folder-select-row">
              <select v-model="weeklyReportParentID">
                <option value="">根目录</option>
                <option v-for="folder in folderOptions" :key="`weekly-folder-${folder.id}`" :value="folder.id">{{ folder.title }}</option>
              </select>
              <button class="btn" type="button" @click="createFolderForTarget('weekly')">新建</button>
            </div>
            <button class="btn primary" :disabled="weeklyReportLoading" @click="generateWeeklyReport">
              {{ weeklyReportLoading ? "生成中..." : "生成周报" }}
            </button>
            <div class="weekly-source-box">
              <input
                ref="weeklyFileInputRef"
                class="hidden-file-input"
                type="file"
                multiple
                accept=".md,.markdown,.txt,.csv,.json,.log"
                @change="onWeeklyFileChange"
              />
              <div class="weekly-source-actions">
                <button class="btn" type="button" @click="weeklyNotePickerOpen = !weeklyNotePickerOpen">
                  选择笔记
                </button>
                <button class="btn" type="button" :disabled="weeklyFileReading" @click="openWeeklyFilePicker">
                  {{ weeklyFileReading ? "读取中..." : "选择本机文件" }}
                </button>
                <span v-if="!weeklySelectedNotes.length && !weeklyLocalFiles.length">未选择来源时，会自动使用最近更新笔记。</span>
                <button v-else class="ghost-btn" type="button" @click="clearWeeklySources">清空来源</button>
              </div>
              <div v-if="weeklySelectedNotes.length || weeklyLocalFiles.length" class="selected-note-row">
                <span v-for="note in weeklySelectedNotes" :key="`weekly-selected-${note.id}`" class="tag-chip">
                  {{ note.title || "未命名" }}
                  <span class="tag-chip-close" @click="toggleWeeklyNote(note)">x</span>
                </span>
                <span v-for="(file, index) in weeklyLocalFiles" :key="`weekly-file-${file.id}`" class="tag-chip file-chip">
                  {{ file.name }}
                  <span class="tag-chip-close" @click="removeWeeklyLocalFile(index)">x</span>
                </span>
              </div>
              <div v-if="weeklyNotePickerOpen" class="recommend-note-picker weekly-note-picker">
                <div class="search-tag-dropdown-head">
                  <strong>选择周报参考笔记</strong>
                  <button class="ghost-btn" @click="weeklyNotePickerOpen = false">完成</button>
                </div>
                <div class="recommend-note-filter">
                  <input
                    v-model="weeklyNoteSearch"
                    type="search"
                    placeholder="搜索笔记标题、路径或标签"
                  />
                </div>
                <div class="recommend-note-list">
                  <button
                    v-for="note in filteredWeeklyNotes"
                    :key="`weekly-note-${note.id}`"
                    class="recommend-note-option"
                    :class="{ active: weeklySelectedIDs.some((id) => Number(id) === Number(note.id)) }"
                    @click="toggleWeeklyNote(note)"
                  >
                    <span class="row-icon note"></span>
                    <span>{{ noteFullPath(note) || note.title || "未命名" }}</span>
                    <small>{{ weeklySelectedIDs.some((id) => Number(id) === Number(note.id)) ? "已选" : "" }}</small>
                  </button>
                  <p v-if="!filteredWeeklyNotes.length" class="muted">没有匹配的笔记。</p>
                </div>
              </div>
            </div>
          </div>
          <div class="weekly-report-result" v-if="weeklyReportResult">
            <template v-if="weeklyReportResult.note">
              <strong>已创建：{{ weeklyReportResult.note.title }}</strong>
              <span>{{ weeklyReportResult.used_ai ? "AI 已参与生成" : "已使用本地整理结果" }}</span>
              <button class="btn" @click="openNote(weeklyReportResult.note.id)">打开周报</button>
            </template>
            <template v-else>
              <strong>生成失败</strong>
              <span>{{ weeklyReportResult.error }}</span>
            </template>
          </div>
        </section>
        <section class="quick-memo-panel page-panel">
          <div class="panel-head">
            <div>
              <small>AI quick memo</small>
              <strong>AI 速记</strong>
            </div>
            <span>选择模板和目标文件夹，语音识别后自动创建笔记</span>
          </div>
          <div class="quick-memo-form">
            <select v-model="quickMemoTemplateKey">
              <option v-for="tpl in workspaceTemplates" :key="`quick-tpl-${tpl.key}`" :value="tpl.key">{{ tpl.name }}</option>
            </select>
            <div class="folder-select-row">
              <select v-model="quickMemoParentID">
                <option value="">根目录</option>
                <option v-for="folder in folderOptions" :key="`quick-folder-${folder.id}`" :value="folder.id">{{ folder.title }}</option>
              </select>
              <button class="btn" type="button" @click="createFolderForTarget('quickMemo')">新建</button>
            </div>
          </div>
          <textarea v-model="quickMemoText" placeholder="语音识别内容会出现在这里，也可以手动输入后生成。"></textarea>
          <div v-if="quickMemoStatusText" class="editor-feedback">
            <span class="feedback-chip" :class="{ warning: quickMemoError }">{{ quickMemoStatusText }}</span>
          </div>
          <div class="quick-memo-actions">
            <button class="btn" :disabled="quickMemoState === 'listening'" @click="startQuickMemoVoice">
              {{ quickMemoState === "paused" ? "继续识别" : "开始语音" }}
            </button>
            <button v-if="quickMemoState === 'listening'" class="btn" @click="pauseQuickMemoVoice">暂停</button>
            <button class="btn primary" :disabled="quickMemoSaving || (!quickMemoText.trim() && !quickMemoInterim.trim())" @click="finishQuickMemoVoice">
              {{ quickMemoSaving ? "保存中..." : "结束并生成笔记" }}
            </button>
          </div>
          <div v-if="quickMemoResult" class="weekly-report-result">
            <strong>已创建：{{ quickMemoResult.title }}</strong>
            <span>已套用 {{ quickMemoTemplate?.name || "默认模板" }}</span>
            <button class="btn" @click="openNote(quickMemoResult.id)">打开笔记</button>
          </div>
        </section>
      </section>

      <section v-else class="note-shell">
        <div class="note-navbar">
          <span class="node">私人</span>
          <template v-for="n in noteNavChain" :key="n.id">
            <span class="sep">/</span>
            <span class="node"><span class="crumb-icon" :class="noteIconClass(n)"></span>{{ n.title || `未命名 #${n.id}` }}</span>
          </template>
        </div>

        <div class="editor-top">
          <span class="muted">{{ noteMode === "edit" ? "编辑模式" : "阅读模式" }}</span>
          <span v-if="noteMode === 'edit'" class="save-state">{{ saveLabel }}</span>
        </div>

        <div v-if="noteMode === 'preview'" class="reader-view">
          <div class="reader-head">
            <div class="reader-meta">
              <h1 class="reader-title">{{ title || "未命名" }}</h1>
              <div v-if="selectedTags.length" class="reader-tags">
                <span v-for="tag in selectedTags" :key="`preview-${tag}`" class="tag-chip static">{{ tag }}</span>
              </div>
            </div>
            <div class="note-actions">
              <button class="btn primary" @click="enterEditMode" :disabled="!selectedId">编辑</button>
              <button class="btn favorite" :class="{ active: hasFavorite }" @click="onFavoriteAction" :disabled="!selectedId">
                {{ hasFavorite ? "取消收藏" : "收藏" }}
              </button>
              <button class="btn" @click="duplicateNote" :disabled="!selectedId">复制</button>
              <button class="btn" @click="exportMarkdown" :disabled="!selectedId">导出 .md</button>
              <button class="btn danger" @click="deleteNote" :disabled="!selectedId">删除</button>
            </div>
          </div>

          <div class="reader-grid">
            <article class="preview reader-preview" v-html="previewHTML" @click="onPreviewClick"></article>

            <aside class="insight-panel">
              <div class="panel-head">
                <div>
                  <small>AI insight</small>
                  <strong>智能洞察</strong>
                </div>
                <div class="panel-tools">
                  <span v-if="noteInsights" class="insight-source-chip" :class="{ ai: noteInsights.used_ai }">
                    {{ noteInsights.used_ai ? "AI 已参与" : "本地规则" }}
                  </span>
                  <button class="info-btn" title="智能洞察说明" @click="openGuide('insight')">!</button>
                  <button class="ghost-btn" @click="refreshNoteIntelligence" :disabled="intelligenceLoading">
                    {{ intelligenceLoading ? "刷新中" : "刷新" }}
                  </button>
                </div>
              </div>

              <div class="insight-section note-status-section">
                <div class="insight-title-row">
                  <h3>笔记属性</h3>
                  <span class="status-pill" :class="selectedNote?.status === 'completed' ? 'completed' : 'unfinished'">
                    {{ noteStatusLabel(selectedNote?.status) }}
                  </span>
                </div>
                <div class="segmented-status">
                  <button :class="{ active: selectedNote?.status !== 'completed' }" @click="setCurrentNoteStatus('unfinished')">未完成</button>
                  <button :class="{ active: selectedNote?.status === 'completed' }" @click="setCurrentNoteStatus('completed')">已完成</button>
                </div>
              </div>

              <div v-if="noteInsights" class="quality-card">
                <div class="quality-ring">
                  <strong>{{ noteInsights.quality_score }}</strong>
                  <span>score</span>
                </div>
                <p>{{ noteInsights.summary }}</p>
              </div>
              <p v-else class="muted">{{ intelligenceLoading ? "正在刷新笔记洞察..." : "点击刷新获取笔记洞察。" }}</p>
              <p v-if="intelligenceMessage" class="tool-message">{{ intelligenceMessage }}</p>

              <div v-if="insightSuggestedTags.length" class="insight-section">
                <div class="insight-title-row">
                  <h3>智能标签</h3>
                  <button class="info-btn small" title="智能标签说明" @click="openGuide('tags')">!</button>
                </div>
                <div class="suggest-tag-row">
                  <button v-for="tag in insightSuggestedTags" :key="`suggest-${tag}`" class="suggest-tag" @click="applySuggestedTag(tag)">
                    + {{ tag }}
                  </button>
                </div>
              </div>

              <div v-if="insightRecommendations.length" class="insight-section">
                <div class="insight-title-row">
                  <h3>内容推荐</h3>
                  <button class="info-btn small" title="内容推荐说明" @click="openGuide('recommend')">!</button>
                </div>
                <button
                  v-for="item in insightRecommendations"
                  :key="`rec-${item.note.id}`"
                  class="insight-item"
                  @click="openNote(item.note.id)"
                >
                  <span>{{ refTitle(item.note) }}</span>
                  <small>{{ item.reason }} 路 {{ scorePercent(item.score) }}</small>
                </button>
              </div>

              <div v-if="insightLinks.backlinks.length || insightLinks.outgoing.length || insightLinks.unlinked_mentions.length" class="insight-section">
                <div class="insight-title-row">
                  <h3>双向链接</h3>
                  <button class="info-btn small" title="双向链接说明" @click="openGuide('backlinks')">!</button>
                </div>
                <button
                  v-for="item in insightLinks.backlinks.slice(0, 4)"
                  :key="`back-${item.id}`"
                  class="insight-item"
                  @click="openNote(item.id)"
                >
                  <span>{{ refTitle(item) }}</span>
                  <small>反向链接</small>
                </button>
                <button
                  v-for="item in insightLinks.unlinked_mentions.slice(0, 4)"
                  :key="`mention-${item.id}`"
                  class="insight-item mention"
                  @click="openNote(item.id)"
                >
                  <span>{{ refTitle(item) }}</span>
                  <small>正文提到但未链接</small>
                </button>
              </div>

              <div v-if="insightOutline.length" class="insight-section">
                <h3>自动大纲</h3>
                <ol class="outline-list">
                  <li v-for="item in insightOutline.slice(0, 6)" :key="`outline-${item}`">{{ item }}</li>
                </ol>
              </div>

              <div v-if="insightQualityIssues.length" class="insight-section">
                <div class="insight-title-row">
                  <h3>质量检查</h3>
                  <button class="info-btn small" title="质量检查说明" @click="openGuide('quality')">!</button>
                </div>
                <div v-for="issue in insightQualityIssues" :key="`${issue.type}-${issue.message}`" class="quality-issue">
                  <strong>{{ issue.message }}</strong>
                  <span>{{ issue.suggestion }}</span>
                </div>
              </div>

              <div v-if="insightDuplicates.length" class="insight-section">
                <h3>相似笔记</h3>
                <button
                  v-for="item in insightDuplicates"
                  :key="`dup-${item.note.id}`"
                  class="insight-item danger-lite"
                  @click="openNote(item.note.id)"
                >
                  <span>{{ refTitle(item.note) }}</span>
                  <small>{{ item.reason }}</small>
                </button>
              </div>
            </aside>
          </div>
        </div>

        <div v-else class="editor-view">
          <input ref="titleInputRef" v-model="title" class="title-input" type="text" placeholder="未命名" />

          <div class="meta-row">
            <div ref="tagMenuRef" class="tag-select">
              <button class="tag-trigger" type="button" @click="tagMenuOpen = !tagMenuOpen">
                <div class="tag-trigger-main">
                  <span v-if="!selectedTags.length" class="tag-placeholder">标签</span>
                  <span v-else class="tag-chip-list">
                    <span v-for="tag in selectedTags" :key="tag" class="tag-chip">
                      {{ tag }}
                      <span class="tag-chip-close" @click.stop="removeTag(tag)">x</span>
                    </span>
                  </span>
                </div>
                <span class="tag-caret" :class="{ open: tagMenuOpen }">v</span>
              </button>

              <div v-if="tagMenuOpen" class="tag-menu">
                <input
                  v-model="tagQuery"
                  type="text"
                  placeholder="搜索或新建标签"
                  @keydown="onTagQueryKeydown"
                />
                <button v-if="canCreateTag" class="tag-option create" type="button" @click="createTagFromQuery">
                  新建标签 "{{ tagQueryTrimmed }}"
                </button>
                <div class="tag-options">
                  <button
                    v-for="tag in filteredTagOptions"
                    :key="`opt-${tag}`"
                    class="tag-option"
                    type="button"
                    @click="toggleTag(tag)"
                  >
                    <span>{{ tag }}</span>
                    <span class="tag-option-right">
                      <span class="check">{{ hasTag(tag) ? "v" : "" }}</span>
                      <span class="tag-delete" @click.stop="deleteTagGlobally(tag)">删除</span>
                    </span>
                  </button>
                  <p v-if="!filteredTagOptions.length && !canCreateTag" class="muted">没有可选标签</p>
                </div>
              </div>
            </div>

            <button class="btn" @click="exitEditMode" :disabled="!selectedId">完成</button>
            <button class="btn favorite" :class="{ active: hasFavorite }" @click="onFavoriteAction" :disabled="!selectedId">
              {{ hasFavorite ? "取消收藏" : "收藏" }}
            </button>
            <button
              class="btn"
              @click="startVoiceInput"
              :disabled="!voiceSupported || aiOptimizing || voiceState === 'listening'"
            >
              {{ voiceState === "paused" ? "继续语音" : "语音输入" }}
            </button>
            <button v-if="voiceState === 'listening'" class="btn" @click="pauseVoiceInput">暂停</button>
            <button
              v-if="voiceState !== 'idle' || voiceTranscript || voiceInterim"
              class="btn"
              @click="finishVoiceInput"
            >
              结束
            </button>
            <button class="btn" @click="optimizeWithAI" :disabled="!selectedId || aiOptimizing">
              {{ aiOptimizing ? "AI 优化中..." : "AI 优化" }}
            </button>
            <button class="btn" @click="duplicateNote" :disabled="!selectedId">复制</button>
            <button class="btn" @click="exportMarkdown" :disabled="!selectedId">导出 .md</button>
            <button class="btn primary" @click="saveNote(false)">保存</button>
            <button class="btn danger" @click="deleteNote" :disabled="!selectedId">删除</button>
          </div>

          <div v-if="voiceStatusText || aiOptimizeMessage || !voiceSupported" class="editor-feedback">
            <span v-if="!voiceSupported" class="feedback-chip warning">当前浏览器不支持语音输入</span>
            <span v-if="voiceStatusText" class="feedback-chip">{{ voiceStatusText }}</span>
            <span v-if="aiOptimizeMessage" class="feedback-chip ai">{{ aiOptimizeMessage }}</span>
          </div>

          <div class="editor-grid">
            <textarea
              ref="editorTextareaRef"
              v-model="markdown"
              placeholder="在这里输入 Markdown，支持 [[页面标题]] 和内部链接。"
              @input="onMarkdownInput"
            ></textarea>
            <article class="preview" v-html="previewHTML" @click="onPreviewClick"></article>
          </div>
        </div>
      </section>
    </main>

    <button v-if="activeView === 'note'" class="ai-fab" @click="aiOpen = !aiOpen">AI</button>
    <section v-if="aiOpen" class="ai-panel">
      <div class="ai-head">
        <strong>本地 AI 助手</strong>
        <button class="ghost-btn" @click="aiOpen = false">关闭</button>
      </div>
      <p class="ai-tip">上方可以查看历史回答，下方输入框可继续追问。</p>
      <div ref="aiMessagesRef" class="ai-history">
        <p v-if="!aiMessages.length" class="ai-empty">可以直接询问你的笔记内容。</p>
        <div v-for="(m, idx) in aiMessages" :key="idx" class="ai-msg" :class="m.role">
          <small>{{ m.role === "user" ? "你" : "AI" }}</small>
          <div class="ai-bubble">{{ m.content }}</div>
        </div>
      </div>
      <div class="ai-input-wrap">
        <textarea
          v-model="aiInput"
          rows="2"
          placeholder="例如：请总结当前笔记的重点"
          @keydown="onAIKeydown"
        />
        <button class="btn primary" :disabled="aiLoading || !aiInput.trim()" @click="askAI">
          {{ aiLoading ? "思考中..." : "提问" }}
        </button>
      </div>
    </section>

    <section
      v-if="contextMenuOpen && contextNote"
      class="context-menu"
      :style="{ left: `${contextMenuX}px`, top: `${contextMenuY}px` }"
      @click.stop
    >
      <div class="context-title">页面</div>
      <button class="context-item" @click="toggleFavoriteFromContext">
        {{ favoriteSet.has(contextNote.id) ? "移出收藏" : "加入收藏" }}
      </button>
      <button class="context-item" @click="renameFromContext">重命名</button>
      <button class="context-item" @click="moveFromContext">移动到</button>
      <button class="context-item" @click="openInNewTabFromContext">在新标签页打开</button>
    </section>

    <section
      v-if="calendarContextMenuOpen"
      class="calendar-context-menu"
      :style="{ left: `${calendarContextMenuX}px`, top: `${calendarContextMenuY}px` }"
      @click.stop
    >
      <button class="context-item" @click="createPlanFromContext">
        <span>创建plan</span>
        <kbd>C</kbd>
      </button>
      <button class="context-item disabled" disabled>
        <span>粘贴plan</span>
        <kbd>{{ isMacLike ? "⌘ V" : "Ctrl V" }}</kbd>
      </button>
    </section>

    <div v-if="guideModalOpen" class="modal-mask" @click.self="closeGuide">
      <section class="guide-modal">
        <div class="guide-head">
          <div>
            <small>Manual</small>
            <strong>{{ currentGuide.title }}</strong>
          </div>
          <button class="ghost-btn" @click="closeGuide">关闭</button>
        </div>
        <div class="guide-body">
          <div class="guide-visual" :class="`guide-${currentGuide.visual}`">
            <div class="guide-screen">
              <span class="guide-dot one"></span>
              <span class="guide-dot two"></span>
              <span class="guide-dot three"></span>
              <span class="guide-line a"></span>
              <span class="guide-line b"></span>
              <span class="guide-card mini-a"></span>
              <span class="guide-card mini-b"></span>
              <span class="guide-card mini-c"></span>
            </div>
          </div>
          <div class="guide-copy">
            <p>{{ currentGuide.subtitle }}</p>
            <ol>
              <li v-for="step in currentGuide.steps" :key="step">{{ step }}</li>
            </ol>
            <div class="guide-tips">
              <strong>鎻愮ず</strong>
              <span v-for="tip in currentGuide.tips" :key="tip">{{ tip }}</span>
            </div>
          </div>
        </div>
      </section>
    </div>

    <div v-if="importModalOpen" class="modal-mask" @click.self="closeImportModal">
      <section class="import-modal">
        <div class="panel-head">
          <div>
            <small>Import</small>
            <strong>从电脑选择文件导入</strong>
          </div>
          <button class="ghost-btn" @click="closeImportModal">关闭</button>
        </div>
        <label class="file-picker">
          <input ref="importFileInputRef" type="file" accept=".md,.markdown,.txt,.csv,.json,.log" @change="onImportFileChange" />
          <span>{{ importReading ? "正在读取..." : (importFileName || "选择本地文件") }}</span>
        </label>
        <input v-model="importTitle" type="text" placeholder="标题，默认使用文件名" />
        <div class="folder-select-row">
          <select v-model="importParentID">
            <option value="">根目录</option>
            <option v-for="folder in folderOptions" :key="folder.id" :value="folder.id">{{ folder.title }}</option>
          </select>
          <button class="btn" type="button" @click="createFolderForTarget('import')">新建文件夹</button>
        </div>
        <div class="template-tag-picker import-tag-picker">
          <button class="tag-trigger" type="button" @click="importTagMenuOpen = !importTagMenuOpen">
            <span v-if="!(importTags || []).length" class="tag-placeholder">选择或新建标签</span>
            <span v-else class="tag-chip-list">
              <span v-for="tag in importTags" :key="`import-modal-tag-${tag}`" class="tag-chip">
                {{ tag }}
                <span class="tag-chip-close" @click.stop="removeImportTag(tag)">x</span>
              </span>
            </span>
            <span class="tag-caret" :class="{ open: importTagMenuOpen }">v</span>
          </button>
          <div v-if="importTagMenuOpen" class="tag-menu template-tag-menu">
            <input v-model="importTagQuery" type="text" placeholder="搜索或新增标签" @keydown.enter.prevent="createImportTagFromQuery" />
            <button v-if="canCreateImportTag" class="tag-option create" type="button" @click="createImportTagFromQuery">
              新增标签 {{ importTagQuery }}
            </button>
            <div class="tag-options">
              <button
                v-for="tag in importTagOptions"
                :key="`import-modal-opt-${tag}`"
                class="tag-option"
                type="button"
                @click="toggleImportTag(tag)"
              >
                <span>{{ tag }}</span>
                <span class="check">{{ importTagSet.has(String(tag).toLowerCase()) ? "v" : "" }}</span>
              </button>
            </div>
          </div>
        </div>
        <textarea v-model="importText" readonly placeholder="选择文件后会在这里预览文本内容"></textarea>
        <div class="import-actions">
          <button class="btn" @click="closeImportModal">取消</button>
          <button class="btn primary" :disabled="!importText.trim() || importReading" @click="submitImportNote">导入笔记系统</button>
        </div>
      </section>
    </div>

    <div v-if="templateModalOpen" class="modal-mask" @click.self="closeTemplateModal">
      <section class="template-modal">
        <div class="panel-head">
          <div>
            <small>Template</small>
            <strong>{{ editingTemplateKey ? "编辑模板" : "新增模板" }}</strong>
          </div>
          <button class="ghost-btn" @click="closeTemplateModal">关闭</button>
        </div>
        <input v-model="templateForm.name" type="text" placeholder="模板名称" />
        <div class="template-tag-picker">
          <button class="tag-trigger" type="button" @click="templateTagMenuOpen = !templateTagMenuOpen">
            <span v-if="!(templateForm.tags || []).length" class="tag-placeholder">选择默认标签</span>
            <span v-else class="tag-chip-list">
              <span v-for="tag in templateForm.tags" :key="`tpl-tag-${tag}`" class="tag-chip">
                {{ tag }}
                <span class="tag-chip-close" @click.stop="removeTemplateTag(tag)">x</span>
              </span>
            </span>
            <span class="tag-caret" :class="{ open: templateTagMenuOpen }">v</span>
          </button>
          <div v-if="templateTagMenuOpen" class="tag-menu template-tag-menu">
            <input v-model="templateTagQuery" type="text" placeholder="搜索或新增标签" @keydown.enter.prevent="createTemplateTagFromQuery" />
            <button v-if="canCreateTemplateTag" class="tag-option create" type="button" @click="createTemplateTagFromQuery">
              新增标签 {{ templateTagQuery }}
            </button>
            <div class="tag-options">
              <button
                v-for="tag in templateTagOptions"
                :key="`tpl-opt-${tag}`"
                class="tag-option"
                type="button"
                @click="toggleTemplateTag(tag)"
              >
                <span>{{ tag }}</span>
                <span class="check">{{ templateTagSet.has(String(tag).toLowerCase()) ? "v" : "" }}</span>
              </button>
            </div>
          </div>
        </div>
        <textarea v-model="templateForm.markdown" placeholder="# 模板 Markdown"></textarea>
        <div class="import-actions">
          <button class="btn" @click="closeTemplateModal">取消</button>
          <button class="btn primary" :disabled="!templateForm.name.trim() || !templateForm.markdown.trim()" @click="saveTemplate">保存模板</button>
        </div>
      </section>
    </div>

    <div v-if="calendarCommandOpen" class="modal-mask command-mask" @click.self="closeCalendarCommand">
      <section class="calendar-command-modal" @keydown="onCalendarCommandKeydown">
        <div class="command-input-row">
          <span class="command-search-icon">⌕</span>
          <input
            ref="calendarCommandInputRef"
            v-model="calendarCommandQuery"
            type="search"
            placeholder="输入命令..."
          />
        </div>
        <div class="command-group-label">日历</div>
        <button
          v-for="(command, index) in calendarCommands"
          :key="command.key"
          class="command-item"
          :class="{ active: index === calendarCommandCursor }"
          type="button"
          @mouseenter="calendarCommandCursor = index"
          @click="runCalendarCommand(command)"
        >
          <span>{{ command.label }}</span>
          <kbd>{{ command.hint }}</kbd>
        </button>
        <div class="command-foot">
          <span>↑↓ 导航</span>
          <span>↵ 选择</span>
          <span>esc 关闭</span>
        </div>
      </section>
    </div>

    <div v-if="calendarPlanSearchOpen" class="modal-mask command-mask" @click.self="closeCalendarPlanSearch">
      <section class="calendar-command-modal calendar-search-modal">
        <div class="command-input-row">
          <span class="command-search-icon">⌕</span>
          <input
            ref="calendarPlanSearchInputRef"
            v-model="calendarPlanSearchQuery"
            type="search"
            placeholder="搜索日历 plan..."
            @keydown="onCalendarPlanSearchKeydown"
          />
        </div>
        <div class="command-group-label">日历 plan</div>
        <button
          v-for="(task, index) in calendarPlanSearchResults"
          :key="`calendar-search-${task.id}`"
          class="command-item calendar-search-result"
          :class="{ active: index === calendarPlanSearchCursor }"
          type="button"
          @mouseenter="calendarPlanSearchCursor = index"
          @click="selectCalendarSearchResult(task)"
        >
          <span>
            <strong>{{ task.title }}</strong>
            <small>{{ task.due || "未设置日期" }} · {{ formatTaskTime(task) }} · {{ priorityLabel(task.priority) }}</small>
          </span>
          <kbd>{{ task.done ? "Done" : "Open" }}</kbd>
        </button>
        <p v-if="!calendarPlanSearchResults.length" class="calendar-search-empty">没有匹配的 plan。</p>
        <div class="command-foot">
          <span>↵ 打开第一个结果</span>
          <span>esc 关闭</span>
        </div>
      </section>
    </div>

    <div v-if="searchModalOpen" class="modal-mask" @click.self="closeSearchModal">
      <section class="search-modal">
        <input
          ref="searchInputRef"
          v-model="searchText"
          type="search"
          placeholder="搜索页面..."
          @input="onSearchModalInput"
          @keydown="onSearchInputKeydown"
        />
        <div class="search-filters">
          <button type="button" :class="{ active: searchSort !== 'relevance' }" @click="cycleSearchSort">
            排序：{{ searchSortLabel }}
          </button>
          <button type="button" :class="{ active: searchOnlyTitle }" @click="toggleSearchOnlyTitle">
            仅标题
          </button>
          <button type="button" :class="{ active: searchInPage }" @click="toggleSearchInPage">
            正文
          </button>
          <button type="button" :class="{ active: searchDate !== 'all' }" @click="cycleSearchDate">
            时间：{{ searchDateLabel }}
          </button>
          <div ref="searchTagMenuRef" class="search-tag-menu">
            <button type="button" class="search-filter-pill" :class="{ active: searchTags.length > 0 }" @click="searchTagMenuOpen = !searchTagMenuOpen">
              {{ searchTagLabel }}
            </button>
            <div v-if="searchTagMenuOpen" class="search-tag-dropdown">
              <div class="search-tag-dropdown-head">
                <strong>选择标签</strong>
                <button type="button" class="ghost-btn" @click="clearSearchTags">清空</button>
              </div>
              <div class="search-tag-dropdown-list">
                <button
                  v-for="tag in searchTagOptions"
                  :key="`search-tag-${tag}`"
                  type="button"
                  class="search-tag-option"
                  :class="{ active: searchTags.some((item) => String(item).toLowerCase() === String(tag).toLowerCase()) }"
                  @click="toggleSearchTag(tag)"
                >
                  <span>{{ tag }}</span>
                  <span class="check">{{ searchTags.some((item) => String(item).toLowerCase() === String(tag).toLowerCase()) ? "已选" : "" }}</span>
                </button>
                <p v-if="!searchTagOptions.length" class="muted">暂无标签</p>
              </div>
            </div>
          </div>
          <span class="search-stat">{{ flatSearchResults.length }} 条结果</span>
        </div>
        <div class="search-list">
          <p v-if="searchLoading" class="muted">搜索中...</p>
          <template v-for="group in searchGroups" :key="group.key">
            <h4>{{ group.label }}</h4>
            <button
              v-for="n in group.items"
              :key="n.id"
              class="search-item"
              :class="{ active: searchActiveNoteID === n.id }"
              @click="selectSearchResult(n)"
            >
              <span class="left">
                <span class="search-title" v-html="searchTitleHTML(n)"></span>
                <span v-if="searchSnippetHTML(n)" class="search-snippet" v-html="searchSnippetHTML(n)"></span>
              </span>
              <span class="right">{{ notePath(n) }} / {{ formatDate(n.updated_at) }}</span>
            </button>
          </template>
          <p v-if="!searchLoading && !searchGroups.length" class="muted">没有匹配结果。</p>
        </div>
      </section>
    </div>
  </div>
</template>



