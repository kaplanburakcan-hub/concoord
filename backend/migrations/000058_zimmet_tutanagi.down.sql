ALTER TABLE saha_tutanaklari DROP COLUMN personel_id;

ALTER TABLE saha_tutanaklari DROP CONSTRAINT saha_tutanaklari_tip_check;
ALTER TABLE saha_tutanaklari ADD CONSTRAINT saha_tutanaklari_tip_check
    CHECK (tip IN ('kaza_yangin_hirsizlik','ek_imalat','mesai','yevmiyeli'));
