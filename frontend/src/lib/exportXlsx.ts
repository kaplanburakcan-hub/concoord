import * as XLSX from "xlsx";

// Listeleri .xlsx olarak indirir (E12). Veri, çağıran ekranın zaten yetki kapsamlı API
// yanıtından gelir; burada yeni veri çekilmez. Yalnız görünen kolonlar yazılır.
export function exportXlsx(
  filename: string,
  sheetName: string,
  columns: { header: string; value: (row: any) => string | number | null | undefined }[],
  rows: any[],
) {
  const aoa = [columns.map((c) => c.header), ...rows.map((r) => columns.map((c) => c.value(r) ?? ""))];
  const ws = XLSX.utils.aoa_to_sheet(aoa);
  const wb = XLSX.utils.book_new();
  XLSX.utils.book_append_sheet(wb, ws, sheetName.slice(0, 31));
  XLSX.writeFile(wb, filename.endsWith(".xlsx") ? filename : `${filename}.xlsx`);
}
