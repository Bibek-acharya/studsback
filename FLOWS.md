# Business Flow Diagrams

---

## Flow 1: Student Registration & Onboarding

```
                         STUDENT REGISTRATION FLOW
                         ════════════════════════

  ┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐
  │ Browser  │     │  Auth    │     │   Auth   │     │   Auth   │
  │ (FE)     │     │ Handler  │     │  Service │     │   Repo   │
  └────┬─────┘     └────┬─────┘     └────┬─────┘     └────┬─────┘
       │                │                │                │
       │  POST /auth/   │                │                │
       │  register      │                │                │
       │ {email,pass,   │                │                │
       │  first_name,   │                │                │
       │  last_name}    │                │                │
       │━━━━━━━━━━━━━━► │                │                │
       │                │  check email   │                │
       │                │  uniqueness ───►                │
       │                │  across all 3  │   FindUserByEmail
       │                │  user tables ──►                │
       │                │◄───────────────┼────────────────│
       │                │                │                │
       │                │ Hash password  │                │
       │                │ (bcrypt)       │                │
       │                │                │                │
       │                │ Generate OTP   │                │
       │                │ Store user +   │                │
       │                │ OTP in-memory  │                │
       │                │ (utils.StoreOTP)│               │
       │                │                │                │
       │  201 Created   │                │                │
       │ {requires_otp: │                │                │
       │  true, email}  │                │                │
       │◄━━━━━━━━━━━━━━┘                │                │
       │                │                │                │
   ┌───┴───┐           │                │                │
   │ USER  │  ⚠️ User  │                │                │
   │ NOT   │  is NOT   │                │                │
   │ PERSI-│  saved to │                │                │
   │ STED  │  DB yet!  │                │                │
   └───┬───┘  In-memory│                │                │
       │      map only │                │                │
       │                │                │                │
       │  POST /auth/   │                │                │
       │  send-otp      │                │                │
       │  {email,       │                │                │
       │  type:"verif" }│                │                │
       │━━━━━━━━━━━━━━► │                │                │
       │                │ Generate OTP   │                │
       │                │ Store in-      │                │
       │                │ memory map     │                │
       │                │                │                │
       │                │ ┌──────────────┴──────────┐     │
       │                │ │     EMAIL QUEUE         │     │
       │                │ │  (Asynq/Redis)          │     │
       │                │ │  OR fallback to console │     │
       │                │ └─────────────────────────┘     │
       │  200 OK (OTP  │                │                │
       │  logged to     │                │                │
       │  console if    │                │                │
       │  SMTP fails)   │                │                │
       │◄━━━━━━━━━━━━━━┘                │                │
       │                │                │                │
       │  POST /auth/   │                │                │
       │  verify-otp    │                │                │
       │  {email, otp}  │                │                │
       │━━━━━━━━━━━━━━► │                │                │
       │                │ Fetch OTP from │                │
       │                │ in-memory map  │                │
       │                │                │                │
       │                │ Validate OTP   │                │
       │                │ (expires 10min)│                │
       │                │                │                │
       │                │ Recover User   │                │
       │                │ data from OTP  │                │
       │                │ store          │                │
       │                │                │                │
       │                │ Check email    │                │
       │                │ uniqueness     │                │
       │                │ again (race) ──►                │
       │                │◄───────────────┼────────────────│
       │                │                │                │
       │                │ CreateUser ───►                │
       │                │  INSERT INTO   │                │
       │                │  users         │                │
       │                │  (finally!)    │                │
       │                │◄───────────────┼────────────────│
       │                │                │                │
       │                │ Generate JWT   │                │
       │                │ (HS256, 24h)   │                │
       │                │                │                │
       │  200 OK        │                │                │
       │  {token, user} │                │                │
       │◄━━━━━━━━━━━━━━┘                │                │
       │                │                │                │
       │  GET /profile  │                │                │
       │  Authorization:│                │                │
       │  Bearer <jwt>  │                │                │
       │━━━━━━━━━━━━━━► │                │                │
       │                │ Validate JWT   │                │
       │                │ (middleware)   │                │
       │                │                │                │
       │                │ FindUserByID   │                │
       │                │ Fetch profile ─►                │
       │                │◄───────────────┼────────────────│
       │  200 OK        │                │                │
       │  {user data}   │                │                │
       │◄━━━━━━━━━━━━━━┘                │                │
       │                │                │                │
       │  POST /prefer- │                │                │
       │  ences         │                │                │
       │  {preferences, │                │                │
       │   preference_  │                │                │
       │   flow, role}  │                │                │
       │━━━━━━━━━━━━━━► │                │                │
       │                │ Update JSONB   │                │
       │                │ preferences    │                │
       │                │ on user ──────►                │
       │                │◄───────────────┼────────────────│
       │  200 OK        │                │                │
       │  {user with    │                │                │
       │   preferences} │                │                │
       │◄━━━━━━━━━━━━━━┘                │                │
```

---

## Flow 2: Institution Registration → Admin Approval → Login

```
               INSTITUTION REGISTRATION & APPROVAL FLOW
               ══════════════════════════════════════════

  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐
  │  FE/QA   │    │   Auth   │    │   Auth   │    │  Super-  │
  │          │    │ Handler  │    │  Service │    │  admin   │
  └────┬─────┘    └────┬─────┘    └────┬─────┘    └────┬─────┘
       │               │               │               │
       │ POST /inst/   │               │               │
       │ auth/register │               │               │
       │ {institution_ │               │               │
       │  name, reg_no,│               │               │
       │  email, ...}  │               │               │
       │━━━━━━━━━━┐   │               │               │
       │          │   │               │               │
       │          │   │  status =     │               │
       │          │   │  "pending"    │               │
       │          │   │  No password  │               │
       │          │   │  set yet      │               │
       │          │   │               │               │
       │          │   │  OTP flow     │               │
       │          │   │  (same as     │               │
       │          │   │  student)     │               │
       │          │   │               │               │
       │◄─────────┘   │               │               │
       │               │               │               │
       │  ╔═══════════════════════════════════╗        │
       │  ║  WAIT: Admin must approve via     ║        │
       │  ║  superadmin panel                 ║        │
       │  ╚═══════════════════════════════════╝        │
       │               │               │               │
       │               │               │         POST /superadmin/
       │               │               │         inst/approve
       │               │               │         {institution_id,
       │               │               │          action:"approved"}
       │               │               │◄───────────────────┘
       │               │               │               │
       │               │               │ Generate random│
       │               │               │ 12-char password│
       │               │               │               │
       │               │               │ Hash password  │
       │               │               │ (bcrypt)       │
       │               │               │               │
       │               │               │ Set status =   │
       │               │               │ "approved"     │
       │               │               │               │
       │               │               │ ┌─────────────┴─────┐
       │               │               │ │  SEND APPROVAL    │
       │               │               │ │  EMAIL (SMTP)     │
       │               │               │ │  Contains:        │
       │               │               │ │  - email          │
       │               │               │ │  - password       │
       │               │               │ └───────────────────┘
       │               │               │               │
       │               │   200 OK      │               │
       │               │◄──────────────┘               │
       │               │               │               │
       │ POST /inst/   │               │               │
       │ auth/login    │               │               │
       │ {email,       │               │               │
       │  password}    │               │               │
       │━━━━━━━━━━━━━► │               │               │
       │               │  Check status │               │
       │               │  = "approved" │               │
       │               │               │               │
       │               │  Verify bcrypt│               │
       │               │  password     │               │
       │               │               │               │
       │               │  Generate JWT │               │
       │               │  (provider_id │               │
       │               │   same as id) │               │
       │               │               │               │
       │               │  Set auth     │               │
       │               │  cookie       │               │
       │  200 OK       │               │               │
       │  {token, user}│               │               │
       │◄━━━━━━━━━━━━━┘               │               │
       │               │               │               │
       │ GET /inst/    │               │               │
       │ dashboard     │               │               │
       │ Authorization:│               │               │
       │ Bearer <jwt>  │               │               │
       │━━━━━━━━━━━━━► │               │               │
       │               │  Validate JWT │               │
       │               │  Check role = │               │
       │               │  "institution"│               │
       │               │               │               │
       │               │  Fetch stats  │               │
       │               │  (programs,   │               │
       │               │   admissions, │               │
       │               │   bookings)   │               │
       │               │               │               │
       │  200 OK       │               │               │
       │  {dashboard}  │               │               │
       │◄━━━━━━━━━━━━━┘               │               │
```

---

## Flow 3: College Claim by Institution

```
                   COLLEGE CLAIM FLOW
                   ═══════════════════

  ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌─────────┐
  │   FE     │   │   Auth   │   │   Auth   │   │  Super-  │   │ College │
  │          │   │  Handler │   │  Service │   │  admin   │   │  Table  │
  └────┬─────┘   └────┬─────┘   └────┬─────┘   └────┬─────┘   └────┬────┘
       │              │              │              │              │
       │  POST /inst/ │              │              │              │
       │  auth/claim  │              │              │              │
       │  {college_id,│              │              │              │
       │   inst_name, │              │              │              │
       │   reg_no,    │              │              │              │
       │   email, ...}│              │              │              │
       │━━━━━━━━━━━━► │              │              │              │
       │              │ Check email  │              │              │
       │              │ uniqueness   │              │              │
       │              │              │              │              │
       │              │ Check reg_no │              │              │
       │              │ uniqueness   │              │              │
       │              │              │              │              │
       │              │ Check college│              │              │
       │              │ NOT already  │              │              │
       │              │ claimed ─────┼──────────────┼─────────────► │
       │              │◄─────────────┼──────────────┼────────────── │
       │              │              │              │              │
       │              │ Generate 12- │              │              │
       │              │ char password│              │              │
       │              │ Hash + store │              │              │
       │              │ status=      │              │              │
       │              │ "pending"    │              │              │
       │              │ claimed=false│              │              │
       │              │              │              │              │
       │              │ OTP flow     │              │              │
       │              │ (verify email)│             │              │
       │  201         │              │              │              │
       │  {requires_  │              │              │              │
       │   otp: true} │              │              │              │
       │◄━━━━━━━━━━━━┘              │              │              │
       │              │              │              │              │
       │  ╔═════════════════════════════════════╗  │              │
       │  ║  SUPERADMIN: Approve claim          ║  │              │
       │  ╚═════════════════════════════════════╝  │              │
       │              │              │              │              │
       │              │              │       POST /superadmin/     │
       │              │              │       institutions/         │
       │              │              │       claim-approve         │
       │              │              │       {institution_id}      │
       │              │              │◄─────────────────┘          │
       │              │              │              │              │
       │              │              │ Copy college  │              │
       │              │              │ data into     │              │
       │              │              │ institution   │              │
       │              │              │ profile:      │              │
       │              │              │  - name       │              │
       │              │              │  - location   │              │
       │              │              │  - website    │              │
       │              │              │  - logo       │              │
       │              │              │  - about      │              │
       │              │              │  - affiliation │             │
       │              │              │  - org_type    │             │
       │              │              │  - verified    │             │
       │              │              │  - email       │             │
       │              │              │  - phone       │             │
       │              │              │              │              │
       │              │              │ Set claimed=  │              │
       │              │              │ true, status= │              │
       │              │              │ "approved"    │              │
       │              │              │              │              │
       │              │              │ Update college│              │
       │              │              │ claimed=true ─┼─────────────►│
       │              │              │◄─────────────┼────────────── │
       │              │              │              │              │
       │              │              │ Reject other │              │
       │              │              │ pending      │              │
       │              │              │ claims for   │              │
       │              │              │ same college │              │
       │              │              │              │              │
       │              │              │ Send approval│              │
       │              │              │ email (SMTP) │              │
       │              │              │ with password│              │
       │              │              │              │              │
       │  200 OK      │              │              │              │
       │◄─────────────┘              │              │              │
```

---

## Flow 4: Scholarship Application with Payment

```
           SCHOLARSHIP APPLICATION & PAYMENT FLOW
           ═══════════════════════════════════════

  ┌──────┐   ┌──────────┐   ┌────────────┐   ┌──────────────┐   ┌────────┐
  │  FE  │   │Scholar-  │   │ Scholar-   │   │  Payment     │   │  DB /  │
  │      │   │ship       │   │ ship       │   │  Gateway     │   │ Email  │
  │      │   │Handler    │   │ Service    │   │  (eSewa)     │   │        │
  └──┬───┘   └────┬─────┘   └─────┬──────┘   └──────┬───────┘   └────┬───┘
     │            │               │                  │               │
     │ GET /educ/ │               │                  │               │
     │ scholar-   │               │                  │               │
     │ ships      │               │                  │               │
     │━━━━━━━━━━━►│  List with    │                  │               │
     │            │  filters,     │                  │               │
     │            │  pagination   │                  │               │
     │            │──────────────►│  GORM query      │               │
     │            │               │──────────────────┼──────────────►│
     │  200 OK    │               │                  │               │
     │  {list,    │               │                  │               │
     │   meta}    │               │                  │               │
     │◄━━━━━━━━━━┘               │                  │               │
     │            │               │                  │               │
     │ POST /educ/ │               │                  │               │
     │ scholar-   │               │                  │               │
     │ ships/:id/ │               │                  │               │
     │ apply      │               │                  │               │
     │  {full_name│               │                  │               │
     │   gender,  │               │                  │               │
     │   email,   │               │                  │               │
     │   see_gpa, │               │                  │               │
     │   ... }    │               │                  │               │
     │━━━━━━━━━━━►│               │                  │               │
     │            │  Can be       │                  │               │
     │            │  anonymous    │                  │               │
     │            │  (no auth)    │                  │               │
     │            │               │                  │               │
     │            │  Validate     │                  │               │
     │            │  fields       │                  │               │
     │            │               │                  │               │
     │            │  INSERT INTO  │                  │               │
     │            │  scholarship_ │                  │               │
     │            │  applications │                  │               │
     │            │  status=      │                  │               │
     │            │  "pending"    │                  │               │
     │            │──────────────►│──────────────────┼──────────────►│
     │            │               │                  │               │
     │            │  Generate     │                  │               │
     │            │  roll_number  │                  │               │
     │            │  (sequence)   │                  │               │
     │            │               │                  │               │
     │  201       │               │                  │               │
     │  {app_id,  │               │                  │               │
     │   roll_    │               │                  │               │
     │   number}  │               │                  │               │
     │◄━━━━━━━━━━┘               │                  │               │
     │            │               │                  │               │
     │  ╔══════════════════════════════════════════╗ │               │
     │  ║  IF PAYMENT REQUIRED:                    ║ │               │
     │  ╚══════════════════════════════════════════╝ │               │
     │            │               │                  │               │
     │ POST /     │               │                  │               │
     │ scholar-   │               │                  │               │
     │ ships/pay/ │               │                  │               │
     │ esewa/     │               │                  │               │
     │ initiate   │               │                  │               │
     │ {app_id,   │               │                  │               │
     │  amount}   │               │                  │               │
     │━━━━━━━━━━━►│──────────────►│──────────────────►│              │
     │            │               │                  │               │
     │            │               │  Generate         │               │
     │            │               │  transaction ref  │               │
     │            │               │                  │               │
     │  200       │               │                  │               │
     │  {esewa_   │               │                  │               │
     │   form_url}│               │                  │               │
     │◄━━━━━━━━━━┘               │                  │               │
     │            │               │                  │               │
     │  FE submits form to eSewa────────────────────►│              │
     │            │               │                  │               │
     │  eSewa redirects back──────┘                  │               │
     │  POST /    │               │                  │               │
     │  esewa/    │               │                  │               │
     │  verify    │               │                  │               │
     │  {refId,   │               │                  │               │
     │   oid,     │               │                  │               │
     │   amt}     │               │                  │               │
     │━━━━━━━━━━━►│──────────────►│──────────────────►│              │
     │            │               │  Verify with      │               │
     │            │               │  eSewa API        │               │
     │            │               │◄─────────────────►│              │
     │            │               │                  │               │
     │            │               │  Update app       │               │
     │            │               │  status="paid"    │               │
     │            │               │──────────────────┼──────────────►│
     │            │               │                  │               │
     │  200 OK    │               │                  │               │
     │  {verified}│               │                  │               │
     │◄━━━━━━━━━━┘               │                  │               │
```

---

## Flow 5: Provider Scholarship — Full Lifecycle

```
         PROVIDER SCHOLARSHIP LIFECYCLE
         ═══════════════════════════════

  ┌─────────┐  ┌──────────────┐  ┌──────────┐  ┌───────────┐  ┌──────────┐
  │Provider │  │   Provider   │  │   Appli- │  │  Written  │  │  Result  │
  │  (FE)   │  │ Scholarship  │  │  cations │  │  Exam     │  │          │
  │         │  │   (CRUD)     │  │          │  │           │  │          │
  └────┬────┘  └──────┬───────┘  └────┬─────┘  └─────┬─────┘  └────┬─────┘
       │              │               │              │             │
       │ POST /       │               │              │             │
       │ provider-    │               │              │             │
       │ scholarships │               │              │             │
       │ {title,      │               │              │             │
       │  desc,       │               │              │             │
       │  deadline,   │               │              │             │
       │  eligibility,│               │              │             │
       │  form_config,│               │              │             │
       │  payment_    │               │              │             │
       │  config,...} │               │              │             │
       │━━━━━━━━━━━►  │               │              │             │
       │              │  INSERT INTO  │              │             │
       │              │  provider_    │              │             │
       │              │  scholarships │              │             │
       │              │  status=draft │              │             │
       │  201         │               │              │             │
       │  {id, slug}  │               │              │             │
       │◄─────────────┘               │              │             │
       │              │               │              │             │
       │  ════════════════════════════════════════════════         │
       │  PHASE 2: Students apply via public endpoint              │
       │  ════════════════════════════════════════════════         │
       │              │               │              │             │
       │              │         POST /public/        │             │
       │              │         volunteer/:id/apply  │             │
       │              │         OR /educ/scholar-    │             │
       │              │         ships/:id/apply      │             │
       │              │◄──────────────┘              │             │
       │              │               │              │             │
       │  GET /       │               │              │             │
       │  provider-   │               │              │             │
       │  apps/       │               │              │             │
       │  applications│               │              │             │
       │  (with       │               │              │             │
       │  filters)    │               │              │             │
       │━━━━━━━━━━━►  │──────────────►│              │             │
       │ 200 {apps}   │               │              │             │
       │◄─────────────┘               │              │             │
       │              │               │              │             │
       │  ════════════════════════════════════════════════         │
       │  PHASE 3: Evaluate applications                            │
       │  ════════════════════════════════════════════════         │
       │              │               │              │             │
       │  PUT /       │               │              │             │
       │  apps/:id/   │               │              │             │
       │  evaluate    │               │              │             │
       │  {score,     │               │              │             │
       │   passed,    │               │              │             │
       │   notes}     │               │              │             │
       │━━━━━━━━━━━►  │──────────────►│              │             │
       │              │  Update app   │              │             │
       │              │  eval_score=  │              │             │
       │              │  eval_passed= │              │             │
       │              │  eval_notes=  │              │             │
       │  200 OK      │               │              │             │
       │◄─────────────┘               │              │             │
       │              │               │              │             │
       │  ════════════════════════════════════════════════         │
       │  PHASE 4: Written exam (optional)                         │
       │  ════════════════════════════════════════════════         │
       │              │               │              │             │
       │  POST /      │               │              │             │
       │  written-    │               │              │             │
       │  exams       │               │              │             │
       │  {title,     │               │              │             │
       │   exam_date, │               │              │             │
       │   total_     │               │              │             │
       │   marks,     │               │              │             │
       │   passing_   │               │              │             │
       │   marks}     │               │              │             │
       │━━━━━━━━━━━►  │──────────────►│─────────────►│             │
       │  201         │               │              │             │
       │◄─────────────┘               │              │             │
       │              │               │              │             │
       │  POST /      │               │              │             │
       │  written-    │               │              │             │
       │  exams/:id/  │               │              │             │
       │  results     │               │              │             │
       │  batch-import│               │              │             │
       │  [{app_id,   │               │              │             │
       │    marks,    │               │              │             │
       │    remarks}] │               │              │             │
       │━━━━━━━━━━━►  │──────────────►│─────────────►│             │
       │              │  Bulk insert  │              │             │
       │              │  written_exam_│              │             │
       │              │  results      │              │             │
       │  200         │               │              │             │
       │◄─────────────┘               │              │             │
       │              │               │              │             │
       │  ════════════════════════════════════════════════         │
       │  PHASE 5: Publish results                                 │
       │  ════════════════════════════════════════════════         │
       │              │               │              │             │
       │  POST /      │               │              │             │
       │  results     │               │              │             │
       │  {title,     │               │              │             │
       │   scholar-   │               │              │             │
       │   ship_id,   │               │              │             │
       │   results:   │               │              │             │
       │   [{name,    │               │              │             │
       │    roll,     │               │              │             │
       │    status}]} │               │              │             │
       │━━━━━━━━━━━►  │──────────────►│─────────────►│             │
       │  201         │               │              │             │
       │◄─────────────┘               │              │             │
       │              │               │              │             │
       │  Students can check          │              │             │
       │  GET /public/results/check?  │              │             │
       │  {roll_number or email}      │              │             │
```

---

## Flow 6: Forum — Post, Comment, Vote

```
            FORUM INTERACTION FLOW
            ═══════════════════════

  ┌──────┐   ┌──────────┐   ┌──────────┐   ┌───────────┐   ┌──────────┐
  │  FE  │   │  Forum   │   │  Forum   │   │  Forum    │   │  Forum   │
  │      │   │ Handler  │   │ Service  │   │   Repo    │   │  Table   │
  └──┬───┘   └────┬─────┘   └────┬─────┘   └─────┬─────┘   └────┬─────┘
     │            │              │                │              │
     │ GET /forum │              │                │              │
     │ communities│              │                │              │
     │━━━━━━━━━━━►│─────────────►│────────────────┼─────────────►│
     │ 200 {list} │              │                │              │
     │◄━━━━━━━━━━┘              │                │              │
     │            │              │                │              │
     │ POST /forum│              │                │              │
     │ communi-   │              │                │              │
     │ ties/:id/  │              │                │              │
     │ join       │              │                │              │
     │ (auth)     │              │                │              │
     │━━━━━━━━━━━►│─────────────►│────────────────┼─────────────►│
     │ 200        │              │  INSERT INTO   │              │
     │            │              │  forum_comm_   │              │
     │            │              │  members       │              │
     │◄━━━━━━━━━━┘              │                │              │
     │            │              │                │              │
     │ POST /forum│              │                │              │
     │ posts      │              │                │              │
     │ (auth)     │              │                │              │
     │ {title,    │              │                │              │
     │  content,  │              │                │              │
     │  category, │              │                │              │
     │  community_│              │                │              │
     │  id,       │              │                │              │
     │  is_poll,  │              │                │              │
     │  poll_     │              │                │              │
     │  options}  │              │                │              │
     │━━━━━━━━━━━►│─────────────►│────────────────┼─────────────►│
     │            │              │  INSERT INTO   │              │
     │            │              │  forum_posts   │              │
     │ 201 {post} │              │                │              │
     │◄━━━━━━━━━━┘              │                │              │
     │            │              │                │              │
     │ POST /forum│              │                │              │
     │ posts/:id/ │              │                │              │
     │ like       │              │                │              │
     │ (auth)     │              │                │              │
     │━━━━━━━━━━━►│─────────────►│────────────────┼─────────────►│
     │            │              │  Check if vote │              │
     │            │              │  exists        │              │
     │            │              │  (unique idx)  │              │
     │            │              │                │              │
     │            │              │  If exists:    │              │
     │            │              │   toggle vote  │              │
     │            │              │   (1 → -1)     │              │
     │            │              │  If not:       │              │
     │            │              │   INSERT vote=1│              │
     │            │              │                │              │
     │            │              │  UPDATE forum_ │              │
     │            │              │  posts SET     │              │
     │            │              │  upvotes =     │              │
     │            │              │  (count)       │              │
     │ 200 {vote} │              │                │              │
     │◄━━━━━━━━━━┘              │                │              │
     │            │              │                │              │
     │ POST /forum│              │                │              │
     │ posts/:id/ │              │                │              │
     │ comments   │              │                │              │
     │ (auth)     │              │                │              │
     │ {content,  │              │                │              │
     │  parent_id?}│             │                │              │
     │━━━━━━━━━━━►│─────────────►│────────────────┼─────────────►│
     │            │              │  INSERT INTO   │              │
     │            │              │  forum_comments│              │
     │            │              │                │              │
     │            │              │  UPDATE forum_ │              │
     │            │              │  posts SET     │              │
     │            │              │  comment_count │              │
     │            │              │  += 1          │              │
     │ 201        │              │                │              │
     │◄━━━━━━━━━━┘              │                │              │
```

---

## Flow 7: Search — Hybrid Vector + Keyword

```
                SEARCH FLOW (Hybrid)
                ════════════════════

  ┌──────┐    ┌──────────┐    ┌─────────────┐    ┌────────┐    ┌──────────┐
  │  FE  │    │  Search  │    │   Search    │    │pgvector│    │ Embedding│
  │      │    │ Handler  │    │   Service   │    │  (DB)  │    │  Service │
  └──┬───┘    └────┬─────┘    └──────┬──────┘    └────┬───┘    └────┬─────┘
     │             │                 │                 │             │
     │ GET /search?│                 │                 │             │
     │ q=engineer+ │                 │                 │             │
     │ &cat=college│                 │                 │             │
     │ &page=1     │                 │                 │             │
     │━━━━━━━━━━━━►│                 │                 │             │
     │             │ Parse params    │                 │             │
     │             │                 │                 │             │
     │             │ Auto-detect     │                 │             │
     │             │ category if     │                 │             │
     │             │ not specified    │                 │             │
     │             │ (keyword map)   │                 │             │
     │             │                 │                 │             │
     │             │ ───────────────►│                 │             │
     │             │                 │                 │             │
     │             │  ┌──────────────────────────────────────┐      │
     │             │  │  EMBEDDING ENABLED?                  │      │
     │             │  │  (check config)                      │      │
     │             │  └──────────┬───────────────────────────┘      │
     │             │             │YES                    │NO       │
     │             │             ▼                       ▼         │
     │             │     ┌──────────────┐       ┌──────────────┐   │
     │             │     │  VECTOR PATH  │       │ KEYWORD ONLY │   │
     │             │     └──────┬───────┘       └──────┬───────┘   │
     │             │            │                       │           │
     │             │     Generate embedding             │           │
     │             │     for query ────────────────────►│           │
     │             │            │                       │           │
     │             │     [0.023, -0.041, ...]           │           │
     │             │            │                       │           │
     │             │     For each table in              │           │
     │             │     category:                      │           │
     │             │     SELECT ...,                    │           │
     │             │       1 - (embedding <=>           │           │
     │             │       query_vec) AS                │           │
     │             │       vector_score                 │           │
     │             │     WHERE embedding IS NOT NULL    │           │
     │             │     ORDER BY vector_score DESC     │           │
     │             │     LIMIT 30                       │           │
     │             │            │                       │           │
     │             │     ┌──────┴──────┐                │           │
     │             │     │ Merge + dedup                │           │
     │             │     │ Apply keyword                │           │
     │             │     │ filter (ILIKE)               │           │
     │             │     │ Paginate                     │           │
     │             │     └──────┬──────┘                │           │
     │             │            │                       │           │
     │             │     ┌──────┴──────┐                │           │
     │             │     │ Sequential ILIKE             │           │
     │             │     │ queries per table            │           │
     │             │     │ LOWER(col) LIKE              │           │
     │             │     │ LOWER('%query%')             │           │
     │             │     │ Merge + paginate             │           │
     │             │     └─────────────┘                │           │
     │             │            │                       │           │
     │             │◄───────────┘                       │           │
     │             │                 │                             │
     │  200 OK     │                 │                             │
     │  {data:     │                 │                             │
     │   {items,   │                 │                             │
     │    category,│                 │                             │
     │    meta:    │                 │                             │
│    {page,    │                 │                             │
     │     limit,   │                 │                             │
     │     total,   │                 │                             │
     │     pages},  │                 │                             │
     │    vector:   │                 │                             │
│    {enabled, │                 │                             │
     │     provider,│                 │                             │
     │     model,   │                 │                             │
     │     dims}}}  │                 │                             │
     │◄━━━━━━━━━━━━┘                 │                             │
```

---

## Flow 8: AI Chat with RAG

```
                   AI CHAT FLOW (RAG)
                   ════════════════════

  ┌──────┐     ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐
  │  FE  │     │    AI    │    │    AI    │    │ Embedding│    │   LLM    │
  │      │     │  Handler │    │  Service │    │ + Search │    │ (Ollama/ │
  │      │     │          │    │          │    │          │    │ OpenAI)  │
  └──┬───┘     └────┬─────┘    └────┬─────┘    └────┬─────┘    └────┬─────┘
     │              │               │               │               │
     │ POST /ai/chat│               │               │               │
     │ SSE stream   │               │               │               │
     │ {message:    │               │               │               │
     │  "Tell me    │               │               │               │
     │   about      │               │               │               │
     │   scholarships│               │               │               │
     │   for girls", │               │               │               │
     │  session_id?} │               │               │               │
     │━━━━━━━━━━━━► │               │               │               │
     │              │──────────────►│               │               │
     │              │               │               │               │
     │              │  1. Embed query──────────────►│              │
     │              │     [0.023, -0.041, ...]      │               │
     │              │     (if LLM_ENABLED)          │               │
     │              │               │               │               │
     │              │  2. Vector search across      │               │
     │              │     8 sources:                │               │
     │              │     colleges, courses,        │               │
     │              │     scholarships, exams,      │               │
     │              │     news, events, blogs,      │               │
     │              │     site_pages                │               │
     │              │               │               │               │
     │              │  3. If no vector results:     │               │
     │              │     fallback to keyword       │               │
     │              │     search                    │               │
     │              │               │               │               │
     │              │  4. Build context from        │               │
     │              │     top results               │               │
     │              │     (truncated to ~8000 chars) │               │
     │              │               │               │               │
     │              │  5. Build system prompt:      │               │
     │              │     "You are StudSphere AI... │               │
     │              │      Use this context to      │               │
     │              │      answer: <context>"       │               │
     │              │               │               │               │
     │              │  6. Send to LLM:              │               │
     │              │     POST /chat/completions    │               │
     │              │     {model, messages:         │               │
     │              │      [system prompt,          │               │
     │              │       history...,             │               │
     │              │       user message]}          │               │
     │              │               │               │               │
     │              │               │───────────────┼──────────────►│
     │              │               │               │               │
     │  SSE stream  │               │               │               │
     │  data: {...} │◄──────────────┤◄──────────────┤◄──────────────┤
     │◄━━━━━━━━━━━━┘               │               │               │
```

---

## Flow 9: Admit Card Generation & Email

```
              ADMIT CARD GENERATION FLOW
              ═══════════════════════════

  ┌──────┐   ┌──────────┐   ┌───────────┐   ┌───────────┐   ┌──────────┐
  │Super-│   │ Scholar- │   │  Email    │   │  Chrome/  │   │  MinIO / │
  │admin │   │ ship     │   │  Queue    │   │  Chromium │   │  Storage │
  │      │   │ Service  │   │ (Asynq)   │   │ (headless)│   │          │
  └──┬───┘   └────┬─────┘   └─────┬─────┘   └─────┬─────┘   └────┬─────┘
     │            │               │               │              │
     │ POST /admin│               │               │              │
     │ /payments/ │               │               │              │
     │ send-admit-│               │               │              │
     │ cards      │               │               │              │
     │  {scholar- │               │               │              │
     │   ship_id, │               │               │              │
     │   app_ids} │               │               │              │
     │━━━━━━━━━━► │               │               │              │
     │            │ Enqueue task  │               │              │
     │            │─────────────►│               │              │
     │            │  type:        │               │              │
     │            │  send_admit   │               │              │
     │            │  _card        │               │              │
     │ 202        │               │               │              │
     │ Accepted   │               │               │              │
     │◄───────────┘               │               │              │
     │            │               │               │              │
     │            │      Worker picks up task:    │              │
     │            │               │               │              │
     │            │               │ For each app: │              │
     │            │               │               │              │
     │            │               │  Render HTML  │              │
     │            │               │  template     │              │
     │            │               │  (admitcard.  │              │
     │            │               │   html)       │              │
     │            │               │               │              │
     │            │               │  Convert to   │              │
     │            │               │  PDF via      │─────────────►│
     │            │               │  chromedp     │              │
     │            │               │  (headless    │              │
     │            │               │   Chromium)   │              │
     │            │               │               │              │
     │            │               │  Upload PDF   │              │
     │            │               │  to MinIO ────┼─────────────►│
     │            │               │  (or local)   │              │
     │            │               │               │              │
     │            │               │  Send email   │              │
     │            │               │  with PDF     │              │
     │            │               │  attachment   │              │
     │            │               │  (SMTP MIME   │              │
     │            │               │   multipart)  │              │
     │            │               │               │              │
```

---

## Flow 10: Google OAuth (All Roles)

```
                   GOOGLE OAUTH FLOW
                   ══════════════════

  ┌──────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐
  │  FE  │    │   Auth   │    │  Google  │    │   Auth   │    │    DB    │
  │      │    │ Handler  │    │   OAuth  │    │  Service │    │          │
  └──┬───┘    └────┬─────┘    └────┬─────┘    └────┬─────┘    └────┬─────┘
     │             │               │               │              │
     │ GET /auth/  │               │               │              │
     │ google?     │               │               │              │
     │ redirect=   │               │               │              │
     │ /dashboard  │               │               │              │
     │━━━━━━━━━━━► │               │               │              │
     │             │ Generate      │               │              │
     │             │ state token   │               │              │
     │             │ (random 32    │               │              │
     │             │  bytes,       │               │              │
     │             │  base64)      │               │              │
     │             │               │               │              │
     │             │ Store in-     │               │              │
     │             │ memory map    │               │              │
     │             │ {state:       │               │              │
     │             │  {redirect,   │               │              │
     │             │   expires:    │               │              │
     │             │   10min}}     │               │              │
     │             │               │               │              │
     │  307        │               │               │              │
     │  Redirect   │               │               │              │
     │  to Google  │               │               │              │
     │◄━━━━━━━━━━━┘               │               │              │
     │             │               │               │              │
     │  Browser redirects to       │               │              │
     │  Google consent screen────► │               │              │
     │             │               │               │              │
     │  User approves              │               │              │
     │             │               │               │              │
     │  Google redirects to        │               │              │
     │  callback with code+state   │               │              │
     │             │               │               │              │
     │ GET /auth/  │               │               │              │
     │ google/     │               │               │              │
     │ callback?   │               │               │              │
     │ code=...&   │               │               │              │
     │ state=...   │               │               │              │
     │━━━━━━━━━━━► │               │               │              │
     │             │ Validate      │               │              │
     │             │ state (CSRF)  │               │              │
     │             │               │               │              │
     │             │ Exchange code │               │              │
     │             │ for token ────►               │              │
     │             │◄──────────────┘               │              │
     │             │               │               │              │
     │             │ Fetch user    │               │              │
     │             │ info from     │               │              │
     │             │ Google API    │               │              │
     │             │ (email, name, │               │              │
     │             │  picture)     │               │              │
     │             │               │               │              │
     │             │──────────────►│               │              │
     │             │               │ Check email   │              │
     │             │               │ across tables │              │
     │             │               │───────────────┼─────────────►│
     │             │               │◄──────────────┼──────────────┤
     │             │               │               │              │
     │             │               │ If new:       │              │
     │             │               │  Create user  │              │
     │             │               │  (role =      │              │
     │             │               │   student)    │              │
     │             │               │               │              │
     │             │               │ If existing:  │              │
     │             │               │  Link google_ │              │
     │             │               │  id to account│              │
     │             │               │               │              │
     │             │               │ Download      │              │
     │             │               │ profile pic   │              │
     │             │               │ to MinIO      │              │
     │             │               │               │              │
     │             │               │ Generate JWT  │              │
     │             │               │ (with claims: │              │
     │             │               │  first_name,  │              │
     │             │               │  last_name,   │              │
     │             │               │  image_url)   │              │
     │             │               │               │              │
     │             │ Set auth      │               │              │
     │             │ cookie        │               │              │
     │             │               │               │              │
     │  307        │               │               │              │
     │  Redirect   │               │               │              │
     │  to FE:     │               │               │              │
     │  /login?    │               │               │              │
     │  token=<jwt>│               │               │              │
     │  &redirect= │               │               │              │
     │  /dashboard │               │               │              │
     │◄━━━━━━━━━━━┘               │               │              │
     │             │               │               │              │
```
