// frappe-gantt kütüphanesi kendi tip tanımını sağlamıyor (ve @types paketi
// yok) — burada yalnızca bu projede kullanılan yüzeyi tipliyoruz.
declare module "frappe-gantt" {
  export interface GanttTask {
    id: string;
    name: string;
    start: string;
    end: string;
    progress?: number;
    dependencies?: string;
    custom_class?: string;
  }

  export interface GanttOptions {
    view_mode?: "Day" | "Week" | "Month" | "Year";
    view_modes?: string[];
    date_format?: string;
    language?: string;
    readonly_dates?: boolean;
    readonly_progress?: boolean;
    bar_height?: number;
    padding?: number;
    on_click?: (task: GanttTask) => void;
    on_date_change?: (task: GanttTask, start: Date, end: Date) => void;
    popup_on?: "click" | "hover";
    custom_popup_html?: ((task: GanttTask) => string) | null;
  }

  export default class Gantt {
    constructor(wrapper: string | HTMLElement, tasks: GanttTask[], options?: GanttOptions);
    refresh(tasks: GanttTask[]): void;
    change_view_mode(mode: string): void;
  }
}
