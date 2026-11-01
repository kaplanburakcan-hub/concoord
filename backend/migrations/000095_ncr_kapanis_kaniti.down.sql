-- NCR kapanış kanıtı — geri alma
DROP TRIGGER IF EXISTS trg_ncr_closed_doc_project ON ncrs;
DROP FUNCTION IF EXISTS check_ncr_closed_doc_project();
ALTER TABLE ncrs DROP COLUMN IF EXISTS closed_document_id;
