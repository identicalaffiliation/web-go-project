INSERT INTO feedbacks (id, product_id, user_id, score, description, created_at)
VALUES
    ('01a10112-b339-715d-8feb-4751a1ced5ef', '01a10112-c339-715d-8feb-4751a1ced5ef', '01b10112-c339-715d-8feb-4751a1ced5ef', 4, 'some desc', now()),
    ('01a10112-c339-715d-8feb-4751a1ced5ef', '01a10112-c339-715d-8feb-4751a1ced5ef', '01b10112-c339-715e-8feb-4751a1ced5ef', 3, NULL, now()),
    ('01a10112-c339-715d-8feb-4751a1cdd5ef', '01a10113-2a74-70cb-843e-ae2f1af18182', '01b10112-c339-715e-8feb-4751a1ced5ef', 3, 'some desc 3', now()),
    ('01a10112-c339-715d-8feb-4751a1ded5ef', '01a10112-c339-715d-8feb-4751a1ced5ef', '01b10113-c339-715e-8feb-4751a1ced5ef', 5, 'some desc 2', now())