-- Seed verisini geri al (sadece DEMO-04 projesi)
DELETE FROM project_design_docs  WHERE project_id = '06802be9-d6e4-4bc8-abb4-6aaa6bfc840f';
DELETE FROM project_personnel    WHERE project_id = '06802be9-d6e4-4bc8-abb4-6aaa6bfc840f';
DELETE FROM supplier_statements  WHERE project_id = '06802be9-d6e4-4bc8-abb4-6aaa6bfc840f';
