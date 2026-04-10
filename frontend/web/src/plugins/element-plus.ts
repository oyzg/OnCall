import type { App } from "vue";
import ElButton from "element-plus/es/components/button/index.mjs";
import "element-plus/es/components/button/style/css.mjs";
import ElCard from "element-plus/es/components/card/index.mjs";
import "element-plus/es/components/card/style/css.mjs";
import ElDescriptions, { ElDescriptionsItem } from "element-plus/es/components/descriptions/index.mjs";
import "element-plus/es/components/descriptions/style/css.mjs";
import ElDrawer from "element-plus/es/components/drawer/index.mjs";
import "element-plus/es/components/drawer/style/css.mjs";
import ElEmpty from "element-plus/es/components/empty/index.mjs";
import "element-plus/es/components/empty/style/css.mjs";
import ElForm, { ElFormItem } from "element-plus/es/components/form/index.mjs";
import "element-plus/es/components/form/style/css.mjs";
import ElInput from "element-plus/es/components/input/index.mjs";
import "element-plus/es/components/input/style/css.mjs";
import ElInputNumber from "element-plus/es/components/input-number/index.mjs";
import "element-plus/es/components/input-number/style/css.mjs";
import ElSelect, { ElOption } from "element-plus/es/components/select/index.mjs";
import "element-plus/es/components/select/style/css.mjs";
import ElTable, { ElTableColumn } from "element-plus/es/components/table/index.mjs";
import "element-plus/es/components/table/style/css.mjs";
import ElTag from "element-plus/es/components/tag/index.mjs";
import "element-plus/es/components/tag/style/css.mjs";
import ElTimeline, { ElTimelineItem } from "element-plus/es/components/timeline/index.mjs";
import "element-plus/es/components/timeline/style/css.mjs";
import ElUpload from "element-plus/es/components/upload/index.mjs";
import "element-plus/es/components/upload/style/css.mjs";

const components = [
  ElButton,
  ElCard,
  ElDescriptions,
  ElDescriptionsItem,
  ElDrawer,
  ElEmpty,
  ElForm,
  ElFormItem,
  ElInput,
  ElInputNumber,
  ElOption,
  ElSelect,
  ElTable,
  ElTableColumn,
  ElTag,
  ElTimeline,
  ElTimelineItem,
  ElUpload,
];

export function installElementPlus(app: App) {
  components.forEach((component) => {
    if (component.name) {
      app.component(component.name, component);
    }
  });
}
