# StudSphere Database Schema & ERD

**Database:** PostgreSQL 16 with pgvector extension  
**Tables:** ~65+ (migrated via GORM AutoMigrate)  
**Last Updated:** 2026-06-22

---

## 1. Entity Relationship Overview

```
┌────────────────────────────────────────────────────────────────────────────┐
│                           USER DOMAIN                                       │
│                                                                             │
│  ┌──────────┐    ┌───────────────────┐    ┌──────────────────────────┐     │
│  │   User   │    │ InstitutionUser   │    │ ScholarshipProviderUser  │     │
│  │ (student)│    │ (institution)     │    │ (scholarship_provider)   │     │
│  └────┬─────┘    └───┬───────────────┘    └──────────┬───────────────┘     │
│       │              │                               │                     │
│       │              ├── CollegeID ──► College       │                     │
│       │              │                               │                     │
│       ▼              ▼                               ▼                     │
│  ┌──────────────────────────────────────────────────────────────────┐      │
│  │                    EducationEntry                                │      │
│  │                    (user_id FK)                                 │      │
│  └──────────────────────────────────────────────────────────────────┘      │
└────────────────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────────────────┐
│                       EDUCATION CONTENT DOMAIN                              │
│                                                                             │
│  ┌──────┐    ┌────────┐    ┌──────┐    ┌──────┐    ┌──────┐               │
│  │ News │    │ Events │    │ Blogs│    │Exams │    │Courses│              │
│  └──────┘    └────────┘    └──────┘    └──────┘    └──────┘              │
│                                                          │                 │
│  ┌──────────────┐    ┌──────────────────┐               │                 │
│  │  University  │◄───│ CollegeUniversity │◄──────────────┘                 │
│  │              │    │ Course (join)    │                                  │
│  └──────┬───────┘    └──────────────────┘                                  │
│         │                                                                  │
│         ▼                                                                  │
│  ┌──────────────┐                                                         │
│  │   College    │──► FeaturedPrograms, Courses, Scholarships (JSONB)      │
│  └──────────────┘                                                         │
└────────────────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────────────────┐
│                     SCHOLARSHIP & APPLICATION DOMAIN                        │
│                                                                             │
│  ┌──────────────┐       ┌─────────────────────┐                            │
│  │  Scholarship │◄──────│ScholarshipApplication│                            │
│  │  (platform)  │       │  (user_id nullable)  │                            │
│  └──────────────┘       └────────┬────────────┘                            │
│                                  │                                          │
│  ┌──────────────────┐           │                                          │
│  │ProviderScholarship│           │  ┌───────────┐                          │
│  │(provider-owned)  │◄──────────┼──│ProviderApp│                          │
│  └──────────────────┘           │  │-lication │                          │
│                                 │  └─────┬─────┘                          │
│  ┌──────────────────┐           │        │                                 │
│  │   WrittenExam    │◄──────────┘        │                                 │
│  └────────┬─────────┘                    ▼                                 │
│           │                     ┌──────────────────┐                       │
│           ▼                     │  ProviderPayment │                       │
│  ┌──────────────────┐           │  (not persisted  │                       │
│  │WrittenExamResult │           │   to DB, virtual)│                       │
│  └──────────────────┘           └──────────────────┘                       │
│                                                                             │
│  ┌──────────────┐    ┌──────────────────┐                                  │
│  │ ProviderInte-│    │  ProviderResult  │                                  │
│  │ rview        │    │  (JSONB results) │                                  │
│  └──────────────┘    └──────────────────┘                                  │
└────────────────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────────────────┐
│                     INSTITUTION PORTAL DOMAIN                               │
│                                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────┐             │
│  │ InstProgram  │    │ InstMedia    │    │InstCounselling   │             │
│  │              │    │              │    │Session           │             │
│  └──────────────┘    └──────────────┘    └────────┬─────────┘             │
│                                                    │                       │
│  ┌──────────────┐    ┌──────────────┐             ▼                       │
│  │ InstEntrance │    │InstEntrance  │    ┌──────────────────┐             │
│  │              │    │Applicant     │    │InstCounselling   │             │
│  └──────────────┘    └──────────────┘    │Booking           │             │
│                                           └──────────────────┘             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────┐             │
│  │ InstEvent    │    │ InstNews     │    │ InstBlog         │             │
│  └──────────────┘    └──────────────┘    └──────────────────┘             │
│                                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────┐             │
│  │ InstQMS      │    │ InstMessage  │    │ AdmissionPage    │             │
│  └──────────────┘    └──────────────┘    └──────────────────┘             │
│                                                                             │
│  ┌──────────────┐    ┌──────────────────┐                                  │
│  │InstSettings  │    │InstSubscription  │  ◄──── InstitutionUser          │
│  └──────────────┘    └──────────────────┘                                  │
└────────────────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────────────────┐
│                    PROVIDER PORTAL DOMAIN (Scholarship Provider)            │
│                                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────┐             │
│  │ ProvNews     │    │ ProvEvent    │    │ ProvBlog         │             │
│  └──────────────┘    └──────────────┘    └──────────────────┘             │
│                                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────┐             │
│  │ ProvService  │    │ ProvSector   │    │ ProvProject      │             │
│  └──────────────┘    └──────────────┘    └──────────────────┘             │
│                                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────┐             │
│  │ ProvGallery  │    │ ProvReview   │    │ ProvVolunteer    │──►VolApp    │
│  └──────────────┘    └──────────────┘    └──────────────────┘             │
│                                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────┐             │
│  │ ProvAccess   │    │ ProvAccessUsr│    │ ProvNotification │             │
│  └──────────────┘    └──────────────┘    └──────────────────┘             │
│                                                                             │
│  ┌──────────────┐    ┌──────────────┐                                      │
│  │ ProvMessage  │    │ ProvCalendar  │                                      │
│  └──────────────┘    │ Event         │                                      │
│                       └──────────────┘                                      │
└────────────────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────────────────┐
│                      FORUM DOMAIN                                           │
│                                                                             │
│  ┌──────────────┐    ┌──────────────────┐                                  │
│  │ ForumPost    │───►│ ForumComment     │  (self-referencing via ParentID) │
│  │              │    └──────────────────┘                                  │
│  │  - UserID ───┼──┐                                                       │
│  │  - Community │  │  ┌──────────────┐    ┌──────────────────┐             │
│  └──────────────┘  └──│ ForumVote    │    │ ForumPollVote    │             │
│                       └──────────────┘    └──────────────────┘             │
│                       ┌──────────────┐                                     │
│                       │ ForumSave    │                                     │
│                       └──────────────┘                                     │
│  ┌──────────────┐    ┌──────────────────┐                                  │
│  │ForumCommunity│───►│ForumCommunity    │                                  │
│  │              │    │Member            │                                  │
│  └──────────────┘    └──────────────────┘                                  │
└────────────────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────────────────┐
│                    STUDENT DASHBOARD DOMAIN                                 │
│                                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────┐             │
│  │ Message      │    │ CalendarEvent│    │ SphereInvite     │             │
│  │(sender_id &  │    │ (user_id FK) │    │ (user_id FK)     │             │
│  │ receiver_id) │    └──────────────┘    └──────────────────┘             │
│  └──────────────┘                                                          │
│  ┌──────────────┐    ┌──────────────────┐                                  │
│  │ Bookmark     │    │ Notification     │                                  │
│  │ (user_id FK) │    │ (user_id FK)     │                                  │
│  └──────────────┘    └──────────────────┘                                  │
└────────────────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────────────────┐
│                      SYSTEM & MISC DOMAIN                                   │
│                                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────┐             │
│  │ ContactInq   │    │ Ad           │    │ CarouselSlide    │             │
│  └──────────────┘    └──────────────┘    └──────────────────┘             │
│                                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────┐             │
│  │ PublicNotif   │    │ SitePage     │    │ Feedback         │             │
│  │              │    │ (chat)       │    │ (user_id FK)     │             │
│  └──────────────┘    └──────────────┘    └──────────────────┘             │
│                                                                             │
│  ┌──────────────┐    ┌──────────────────┐                                  │
│  │ Review       │───►│ ReviewHelpful    │  (user_id + review_id unique)    │
│  │(college_id)  │    └──────────────────┘                                  │
│  │(user_id FK)  │    ┌──────────────────┐                                  │
│  └──────────────┘    │ ReviewReport     │                                  │
│                       └──────────────────┘                                  │
│                                                                             │
│  ┌──────────────────────┐                                                   │
│  │ ShikshaApplication   │  (Project Shiksha - standalone)                  │
│  └──────────────────────┘                                                   │
└────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Complete Table Reference

### 2.1 User Domain (3 separate user tables)

#### `users` — Student Users

| Column        | Type         | Constraints       | Notes                            |
| ------------- | ------------ | ----------------- | -------------------------------- |
| id            | uint         | PK                |                                  |
| created_at    | timestamptz  |                   |                                  |
| updated_at    | timestamptz  |                   |                                  |
| deleted_at    | timestamptz  | INDEX             | Soft delete                      |
| email         | varchar(255) | UNIQUE, NOT NULL  | Login identifier                 |
| password      | varchar(255) | NULLABLE          | bcrypt hash; null for OAuth-only |
| first_name    | varchar(255) | NOT NULL          |                                  |
| last_name     | varchar(255) | NOT NULL          |                                  |
| phone         | varchar(255) | DEFAULT ''        |                                  |
| date_of_birth | varchar(255) | DEFAULT ''        | Stored as string                 |
| gender        | varchar(255) | DEFAULT ''        |                                  |
| nationality   | varchar(255) | DEFAULT ''        |                                  |
| address       | text         | DEFAULT ''        |                                  |
| bio           | text         | DEFAULT ''        |                                  |
| google_id     | varchar(255) | UNIQUE, NULLABLE  | Google OAuth subject ID          |
| image_url     | varchar(255) | DEFAULT ''        | Profile picture                  |
| role          | varchar(255) | DEFAULT 'student' |                                  |
| status        | varchar(255) | DEFAULT 'active'  | active / suspended               |
| last_login_at | timestamptz  | NULLABLE          |                                  |
| preferences   | jsonb        | DEFAULT NULL      | Onboarding preferences           |

**Indexes:** `idx_users_email` (UNIQUE), `idx_users_google_id` (UNIQUE), `idx_users_deleted_at`

#### `institution_users` — Institution Users

| Column                     | Type         | Constraints           | Notes                                     |
| -------------------------- | ------------ | --------------------- | ----------------------------------------- |
| id                         | uint         | PK                    |                                           |
| created_at                 | timestamptz  |                       |                                           |
| updated_at                 | timestamptz  |                       |                                           |
| deleted_at                 | timestamptz  | INDEX                 |                                           |
| institution_name           | varchar(255) | NOT NULL              |                                           |
| registration_number        | varchar(255) | UNIQUE, NOT NULL      | Gov registration                          |
| email                      | varchar(255) | UNIQUE, NOT NULL      |                                           |
| google_id                  | varchar(255) | UNIQUE, NULLABLE      |                                           |
| password                   | varchar(255) | NULLABLE              | Set on approval                           |
| role                       | varchar(255) | DEFAULT 'institution' |                                           |
| status                     | varchar(255) | DEFAULT 'pending'     | pending / approved / rejected / suspended |
| contact_number             | varchar(255) | DEFAULT ''            |                                           |
| contact_email              | varchar(255) | DEFAULT ''            | Public contact email                      |
| contact_phone              | varchar(255) | DEFAULT ''            |                                           |
| province                   | varchar(255) | DEFAULT ''            |                                           |
| district                   | varchar(255) | DEFAULT ''            |                                           |
| local_body                 | varchar(255) | DEFAULT ''            |                                           |
| organization_type          | varchar(255) | DEFAULT ''            |                                           |
| pan_number                 | varchar(255) | DEFAULT ''            |                                           |
| website_url                | varchar(255) | DEFAULT ''            |                                           |
| contact_person             | varchar(255) | DEFAULT ''            |                                           |
| contact_person_designation | varchar(255) | DEFAULT ''            |                                           |
| contact_person_phone       | varchar(255) | DEFAULT ''            |                                           |
| rejection_reason           | varchar(255) | DEFAULT ''            |                                           |
| profile_access             | jsonb        | DEFAULT '{}'          | Visibility toggles per section            |
| logo_url                   | varchar(255) | DEFAULT ''            |                                           |
| banner_url                 | varchar(255) | DEFAULT ''            |                                           |
| about                      | text         |                       |                                           |
| vision                     | text         |                       |                                           |
| mission                    | text         |                       |                                           |
| college_id                 | uint         | DEFAULT 0             | FK to `colleges` (if claimed)             |
| level                      | varchar(255) | DEFAULT ''            | School/+2/Bachelor/etc                    |
| affiliation                | varchar(255) | DEFAULT ''            | University affiliation                    |
| claimed                    | boolean      | DEFAULT false         | College claimed?                          |
| verified                   | boolean      | DEFAULT false         | Admin-verified badge                      |
| verified_by                | varchar(255) | DEFAULT ''            |                                           |
| verified_at                | timestamptz  | NULLABLE              |                                           |
| profile_data               | jsonb        | DEFAULT '{}'          | Complex profile sections                  |
| featured                   | boolean      | DEFAULT false         | Featured on landing                       |
| facebook_url               | varchar(255) | DEFAULT ''            |                                           |
| instagram_url              | varchar(255) | DEFAULT ''            |                                           |
| tiktok_url                 | varchar(255) | DEFAULT ''            |                                           |
| youtube_url                | varchar(255) | DEFAULT ''            |                                           |
| linkedin_url               | varchar(255) | DEFAULT ''            |                                           |
| map_url                    | varchar(255) | DEFAULT ''            |                                           |

**Relationships:**

- `institution_users.college_id` → `colleges.id` (many-to-one, optional)
- `institution_users.id` → `institution_subscriptions.institution_id` (one-to-one)
- `institution_users.id` → Institution-owned content tables (programs, media, etc.)

#### `scholarship_provider_users` — Provider Users

| Column              | Type         | Constraints                    | Notes                         |
| ------------------- | ------------ | ------------------------------ | ----------------------------- |
| id                  | uint         | PK                             |                               |
| created_at          | timestamptz  |                                |                               |
| updated_at          | timestamptz  |                                |                               |
| deleted_at          | timestamptz  | INDEX                          |                               |
| provider_name       | varchar(255) | NOT NULL                       |                               |
| registration_number | varchar(255) | UNIQUE, NOT NULL               |                               |
| email               | varchar(255) | UNIQUE, NOT NULL               |                               |
| contact_number      | varchar(255) |                                |                               |
| pan_number          | varchar(255) |                                |                               |
| website_url         | varchar(255) |                                |                               |
| logo_url            | varchar(255) | NULLABLE                       |                               |
| address             | varchar(255) | DEFAULT ''                     |                               |
| about_text          | text         | DEFAULT ''                     |                               |
| mission             | text         | DEFAULT ''                     |                               |
| values              | text         | DEFAULT ''                     |                               |
| google_id           | varchar(255) | UNIQUE, NULLABLE               |                               |
| password            | varchar(255) | NULLABLE                       |                               |
| status              | varchar(255) | DEFAULT 'pending'              | pending / approved / rejected |
| role                | varchar(255) | DEFAULT 'scholarship_provider' |                               |
| founder_name        | varchar(255) | DEFAULT ''                     |                               |
| founder_role        | varchar(255) | DEFAULT ''                     |                               |
| founder_message     | text         | DEFAULT ''                     |                               |
| founder_image_url   | varchar(255) | DEFAULT ''                     |                               |
| facebook_url        | varchar(255) | DEFAULT ''                     |                               |
| instagram_url       | varchar(255) | DEFAULT ''                     |                               |
| youtube_url         | varchar(255) | DEFAULT ''                     |                               |
| linkedin_url        | varchar(255) | DEFAULT ''                     |                               |
| map_url             | text         | DEFAULT ''                     |                               |
| brochure_url        | varchar(255) | DEFAULT ''                     |                               |
| banner_url          | varchar(255) | DEFAULT ''                     |                               |

**Relationships:**

- `scholarship_provider_users.id` → `provider_scholarships.provider_id`
- `scholarship_provider_users.id` → Provider-owned content (news, events, blogs, etc.)

#### `education_entries` — User Education History

FK to `users.id`. One user can have many entries.

---

### 2.2 Core Content Domain

#### `colleges`

| Column             | Type         | Constraints          | Notes                    |
| ------------------ | ------------ | -------------------- | ------------------------ |
| id                 | uint         | PK                   |                          |
| university_id      | uint         | INDEX                | FK to `universities`     |
| name               | varchar(255) | NOT NULL, INDEX      |                          |
| full_name          | text         |                      |                          |
| location           | varchar(255) | NOT NULL             |                          |
| affiliation        | varchar(255) |                      |                          |
| college_type       | varchar(255) |                      | Private/Public/Community |
| verified           | boolean      | DEFAULT false        |                          |
| claimed            | boolean      | DEFAULT false        |                          |
| popular            | boolean      | DEFAULT false        |                          |
| featured           | boolean      | DEFAULT false, INDEX |                          |
| rating             | float        |                      | Average rating           |
| reviews            | int          |                      | Review count             |
| programs           | int          |                      | Program count            |
| established        | varchar(255) |                      |                          |
| students           | varchar(255) |                      |                          |
| description        | text         |                      |                          |
| website            | varchar(255) |                      |                          |
| email              | varchar(255) |                      |                          |
| phone              | varchar(255) |                      |                          |
| image_url          | varchar(255) |                      |                          |
| featured_programs  | jsonb        |                      |                          |
| amenities          | jsonb        |                      |                          |
| courses            | jsonb        |                      | Inline course data       |
| scholarships       | jsonb        |                      | Inline scholarship data  |
| gallery            | jsonb        |                      |                          |
| programs_list      | jsonb        |                      |                          |
| about              | jsonb        |                      | Multiple sections        |
| admissions         | jsonb        |                      | Admission details        |
| admission_cards    | jsonb        |                      |                          |
| offered_programs   | jsonb        |                      |                          |
| alumni             | jsonb        |                      |                          |
| departments        | jsonb        |                      |                          |
| college_reviews    | jsonb        |                      | Inline reviews           |
| academic_fit_score | int          | DEFAULT 5            | For recommender          |
| campus_life_score  | int          | DEFAULT 5            |                          |
| career_fit_score   | int          | DEFAULT 5            |                          |
| balanced_fit_score | int          | DEFAULT 5            |                          |
| profile_tags       | jsonb        |                      |                          |
| embedding          | vector(1536) | NULLABLE             | pgvector                 |

**Relationships:**

- `colleges.university_id` → `universities.id` (many-to-one)

#### `universities`

| Column          | Type                             | Notes       |
| --------------- | -------------------------------- | ----------- |
| id              | uint PK                          |             |
| name            | varchar(255) UNIQUE NOT NULL     |             |
| logo            | varchar(255)                     |             |
| location        | varchar(255)                     |             |
| type            | varchar(255)                     |             |
| is_nepali       | boolean DEFAULT true             |             |
| rank            | int                              |             |
| rating          | float                            |             |
| review_count    | int                              |             |
| verified        | boolean DEFAULT false            |             |
| popular         | boolean DEFAULT false            |             |
| status          | varchar(255) DEFAULT 'published' |             |
| description     | text                             |             |
| established     | varchar(255)                     |             |
| students        | varchar(255)                     |             |
| chancellor      | varchar(255)                     |             |
| vice_chancellor | varchar(255)                     |             |
| founder         | varchar(255)                     |             |
| website         | varchar(255)                     |             |
| cover           | varchar(255)                     | Cover image |
| about           | jsonb                            |             |
| contact         | jsonb                            |             |
| quick           | jsonb                            | Quick facts |
| overview        | jsonb                            |             |
| leadership      | jsonb                            |             |
| courses         | jsonb                            |             |
| programs        | jsonb                            |             |
| scholarships    | jsonb                            |             |
| events          | jsonb                            |             |
| news            | jsonb                            |             |
| downloads       | jsonb                            |             |
| gallery         | jsonb                            |             |
| faculties       | jsonb                            |             |
| admissions      | jsonb                            |             |
| reviews         | jsonb                            |             |

#### `college_university_courses` — Join Table

| Column        | Type         | Constraints                                   |
| ------------- | ------------ | --------------------------------------------- |
| id            | uint         | PK                                            |
| college_id    | uint         | NOT NULL, INDEX, UNIQUE(college, uni, course) |
| university_id | uint         | NOT NULL, INDEX, UNIQUE(college, uni, course) |
| course_id     | uint         | NOT NULL, INDEX, UNIQUE(college, uni, course) |
| status        | varchar(255) |                                               |

#### `courses`

| Column         | Type                  | Notes                 |
| -------------- | --------------------- | --------------------- |
| id             | uint PK               |                       |
| title          | varchar(255) NOT NULL |                       |
| short_title    | varchar(255)          |                       |
| colleges_count | int                   | Denormalized count    |
| affiliation    | varchar(255)          |                       |
| badges         | jsonb                 |                       |
| level          | varchar(255)          |                       |
| field          | varchar(255)          |                       |
| duration       | varchar(255)          |                       |
| est_fee        | varchar(255)          | Estimated fee         |
| highlights     | jsonb                 |                       |
| career_path    | varchar(255)          |                       |
| description    | text                  |                       |
| location       | varchar(255)          |                       |
| govt_fee       | varchar(255)          |                       |
| private_fee    | varchar(255)          |                       |
| mode           | varchar(255)          | Online/offline/hybrid |
| degree_label   | varchar(255)          |                       |
| about          | jsonb                 |                       |
| curriculum     | jsonb                 |                       |
| admissions     | jsonb                 |                       |
| careers        | jsonb                 |                       |
| embedding      | vector(1536)          | NULLABLE              |

#### `exams`

| Column        | Type                  | Notes             |
| ------------- | --------------------- | ----------------- |
| id            | uint PK               |                   |
| slug          | varchar(255) UNIQUE   |                   |
| title         | varchar(255) NOT NULL |                   |
| board         | varchar(255)          |                   |
| badges        | jsonb                 |                   |
| level         | varchar(255)          |                   |
| type          | varchar(255)          |                   |
| exam_date     | varchar(255)          | BS date string    |
| exam_date_ad  | timestamptz           | AD date           |
| form_deadline | varchar(255)          |                   |
| fee           | varchar(255)          |                   |
| highlights    | jsonb                 |                   |
| description   | text                  |                   |
| status        | varchar(255)          |                   |
| image_url     | varchar(255)          |                   |
| university    | varchar(255)          |                   |
| faculty       | varchar(255)          |                   |
| nepali_date   | varchar(255)          |                   |
| overview      | text                  |                   |
| weightage     | jsonb                 | Mark distribution |
| timeline      | jsonb                 |                   |
| notices       | jsonb                 |                   |
| faqs          | jsonb                 |                   |
| embedding     | vector(1536)          | NULLABLE          |

#### `news`

| Column    | Type                  |
| --------- | --------------------- |
| id        | uint PK               |
| category  | varchar(255)          |
| title     | varchar(255) NOT NULL |
| excerpt   | text                  |
| content   | text                  |
| image     | varchar(255)          |
| author    | varchar(255)          |
| date      | varchar(255)          |
| read_time | varchar(255)          |
| source    | varchar(255)          |
| tags      | jsonb                 |
| embedding | vector(1536) NULLABLE |

#### `events`

| Column           | Type                         |
| ---------------- | ---------------------------- |
| id               | uint PK                      |
| title            | varchar(255) NOT NULL        |
| excerpt          | text                         |
| description      | text                         |
| category         | varchar(255)                 |
| organizer        | varchar(255)                 |
| location         | varchar(255)                 |
| date             | varchar(255)                 |
| time             | varchar(255)                 |
| registration_fee | varchar(255)                 |
| image            | varchar(255)                 |
| interested       | int                          |
| trending         | boolean                      |
| featured         | boolean DEFAULT false, INDEX |
| embedding        | vector(1536) NULLABLE        |

#### `blogs`

| Column    | Type                  |
| --------- | --------------------- |
| id        | uint PK               |
| title     | varchar(255) NOT NULL |
| slug      | varchar(255) UNIQUE   |
| excerpt   | text                  |
| content   | text                  |
| image     | varchar(255)          |
| author    | varchar(255)          |
| category  | varchar(255)          |
| tags      | jsonb                 |
| read_time | varchar(255)          |
| featured  | boolean DEFAULT false |
| published | boolean DEFAULT true  |
| views     | int DEFAULT 0         |
| embedding | vector(1536) NULLABLE |

#### `blog_comments`

| Column  | Type                  | Notes            |
| ------- | --------------------- | ---------------- |
| id      | uint PK               |                  |
| blog_id | uint NOT NULL, INDEX  | FK to `blogs`    |
| author  | varchar(255) NOT NULL | Name (anonymous) |
| avatar  | varchar(255)          |                  |
| message | text NOT NULL         |                  |
| likes   | int DEFAULT 0         |                  |

#### `entrances` (via education module)

Public entrance exam listings accessible via `/entrances` endpoints.

---

### 2.3 Scholarship Domain

#### `scholarships` — Platform Scholarships

~75+ columns. Key fields:

| Column                      | Type                         | Notes                                 |
| --------------------------- | ---------------------------- | ------------------------------------- |
| id                          | uint PK                      |                                       |
| slug                        | varchar(255) UNIQUE          | URL-friendly                          |
| title                       | varchar(255) NOT NULL        |                                       |
| provider                    | varchar(255) NOT NULL        | Organization name                     |
| provider_name               | varchar(255)                 |                                       |
| location                    | varchar(255)                 |                                       |
| value                       | varchar(255)                 | Display amount                        |
| deadline                    | timestamptz                  |                                       |
| degree_level                | varchar(255)                 |                                       |
| funding_type                | varchar(255)                 | Full/Partial                          |
| scholarship_type            | varchar(255)                 | Merit/Need/Other                      |
| description                 | text                         |                                       |
| image_url                   | varchar(255)                 |                                       |
| banner_background_image_url | varchar(255)                 |                                       |
| field_of_study              | jsonb                        |                                       |
| total_seats                 | int                          |                                       |
| amount_per_student          | float                        |                                       |
| application_start_date      | timestamptz                  |                                       |
| result_publication_date     | timestamptz                  |                                       |
| selection_process           | jsonb                        |                                       |
| eligibility_criteria        | jsonb                        |                                       |
| excluded_regions            | jsonb                        |                                       |
| required_documents          | jsonb                        |                                       |
| timeline                    | jsonb                        |                                       |
| benefits                    | jsonb                        |                                       |
| faqs                        | jsonb                        |                                       |
| status                      | varchar(255) DEFAULT 'draft' |                                       |
| form_config                 | jsonb                        | Dynamic form fields                   |
| payment_config              | jsonb                        | Fee structure                         |
| bank_account_name           | varchar(255)                 |                                       |
| bank_account_no             | varchar(255)                 |                                       |
| bank_name                   | varchar(255)                 |                                       |
| bank_branch                 | varchar(255)                 |                                       |
| provider_scholarship_id     | uint NULLABLE, INDEX         | Link to provider table                |
| exam_date                   | varchar(255) DEFAULT ''      |                                       |
| exam_time                   | varchar(255) DEFAULT ''      |                                       |
| slug                        | varchar(255)                 |                                       |
| short_desc                  | text DEFAULT ''              |                                       |
| institution_id              | int DEFAULT 0                |                                       |
| apply_link                  | varchar(255)                 | External application                  |
| coverage_area               | varchar(255)                 |                                       |
| contact_email               | varchar(255)                 |                                       |
| primary_phone               | varchar(255)                 |                                       |
| secondary_phone             | varchar(255)                 |                                       |
| website_url                 | varchar(255)                 |                                       |
| office_address              | varchar(255)                 |                                       |
| map_url                     | varchar(255)                 |                                       |
| ~15 more JSONB fields       | jsonb                        | Gallery, partners, FAQ sections, etc. |
| embedding                   | vector(1536) NULLABLE        |                                       |

#### `scholarship_applications`

| Column                  | Type                           | Notes                              |
| ----------------------- | ------------------------------ | ---------------------------------- |
| id                      | uint PK                        |                                    |
| scholarship_id          | uint, INDEX                    | FK to `scholarships`               |
| user_id                 | uint NULLABLE, INDEX           | Null for anonymous                 |
| full_name               | varchar NOT NULL               |                                    |
| gender                  | varchar NOT NULL               |                                    |
| ethnicity               | varchar(255)                   |                                    |
| ethnicity_other         | varchar(255)                   |                                    |
| date_of_birth_bs        | varchar(255)                   |                                    |
| date_of_birth_ad        | timestamptz                    |                                    |
| age                     | int                            |                                    |
| phone_number            | varchar(255)                   |                                    |
| email                   | varchar(255)                   |                                    |
| photo_url               | varchar(255)                   |                                    |
| see_gpa                 | varchar(255)                   | SEE GPA                            |
| school_type             | varchar(255)                   |                                    |
| school_name             | varchar(255)                   |                                    |
| school_province         | varchar(255)                   |                                    |
| school_district         | varchar(255)                   |                                    |
| school_municipality     | varchar(255)                   |                                    |
| school_tole             | varchar(255)                   |                                    |
| documents               | jsonb                          | Uploaded docs                      |
| permanent_province      | varchar(255)                   |                                    |
| permanent_district      | varchar(255)                   |                                    |
| permanent_municipality  | varchar(255)                   |                                    |
| permanent_ward          | varchar(255)                   |                                    |
| permanent_tole          | varchar(255)                   |                                    |
| temporary_province      | varchar(255)                   |                                    |
| temporary_district      | varchar(255)                   |                                    |
| temporary_municipality  | varchar(255)                   |                                    |
| temporary_ward          | varchar(255)                   |                                    |
| temporary_tole          | varchar(255)                   |                                    |
| guardian_name           | varchar(255)                   |                                    |
| guardian_phone          | varchar(255)                   |                                    |
| guardian_email          | varchar(255)                   |                                    |
| father_occupation       | varchar(255)                   |                                    |
| father_occupation_other | varchar(255)                   |                                    |
| mother_occupation       | varchar(255)                   |                                    |
| mother_occupation_other | varchar(255)                   |                                    |
| family_monthly_income   | float                          |                                    |
| family_members_count    | int                            |                                    |
| stream                  | varchar(255)                   |                                    |
| exam_center             | varchar(255)                   |                                    |
| personal_statement      | text                           |                                    |
| roll_number             | varchar(20) DEFAULT ''         | Auto-generated                     |
| status                  | varchar(255) DEFAULT 'pending' | pending/reviewed/accepted/rejected |
| roll_number             | text DEFAULT ''                |                                    |
| see_gpa                 | text                           | (via ALTER TABLE)                  |

**Payment** is NOT persisted as a separate DB table. The `Payment` and `ProviderPayment` structs are virtual (`gorm:"-"`) and used at runtime.

#### `provider_scholarships` — Provider-Owned Scholarships

~70+ columns (mirrors `scholarships` with `provider_id` instead).

| Column      | Type                  | Notes                              |
| ----------- | --------------------- | ---------------------------------- |
| id          | uint PK               |                                    |
| provider_id | uint, INDEX, NOT NULL | FK to `scholarship_provider_users` |
| slug        | varchar(255) UNIQUE   |                                    |
| title       | varchar(255) NOT NULL |                                    |
| ...         | ~65 more              | Same as `scholarships`             |
| slug        | varchar(255)          |                                    |

#### `provider_applications`

| Column                     | Type                           | Notes                                         |
| -------------------------- | ------------------------------ | --------------------------------------------- |
| id                         | uint PK                        |                                               |
| scholarship_id             | uint, INDEX                    | FK to `provider_scholarships`                 |
| user_id                    | uint NULLABLE, INDEX           | Null for anonymous                            |
| full_name                  | varchar(255)                   |                                               |
| first_name                 | varchar(255)                   |                                               |
| last_name                  | varchar(255)                   |                                               |
| email                      | varchar(255)                   |                                               |
| phone_number               | varchar(255)                   |                                               |
| gender                     | varchar(255)                   |                                               |
| ...                        | ~45 more                       | Same app fields as `scholarship_applications` |
| status                     | varchar(255) DEFAULT 'pending' |                                               |
| evaluation_score           | int NULLABLE                   |                                               |
| evaluation_passed          | boolean DEFAULT false          |                                               |
| evaluation_notes           | text                           |                                               |
| scholarship_application_id | uint NULLABLE                  | Link to platform app                          |
| roll_number                | varchar(20)                    |                                               |
| see_gpa                    | text                           | (via ALTER TABLE)                             |
| rejection_reason           | text                           |                                               |

#### `payments` — (defined but virtual, `gorm:"-"`)

Not a DB table. Payment data is transient — processed at runtime via eSewa/Khalti.

#### `provider_interviews`

| Column         | Type                        |
| -------------- | --------------------------- |
| id             | uint PK                     |
| application_id | uint, INDEX                 |
| provider_id    | uint, INDEX                 |
| scheduled_at   | timestamptz                 |
| duration       | int (minutes)               |
| type           | varchar (online/in-person)  |
| location       | varchar                     |
| link           | varchar                     |
| status         | varchar DEFAULT 'scheduled' |
| notes          | text                        |

#### `written_exams`

| Column         | Type                    |
| -------------- | ----------------------- |
| id             | uint PK                 |
| provider_id    | uint, INDEX, NOT NULL   |
| scholarship_id | uint, INDEX, NOT NULL   |
| title          | varchar NOT NULL        |
| exam_date      | varchar                 |
| duration       | int (minutes)           |
| location       | varchar                 |
| total_marks    | int                     |
| passing_marks  | int                     |
| status         | varchar DEFAULT 'draft' |

#### `written_exam_results`

| Column             | Type    | Constraints                               |
| ------------------ | ------- | ----------------------------------------- |
| id                 | uint PK |                                           |
| written_exam_id    | uint    | UNIQUE(exam_id, application_id), NOT NULL |
| application_id     | uint    | UNIQUE(exam_id, application_id), NOT NULL |
| marks_obtained     | int     |                                           |
| remarks            | varchar |                                           |
| interview_location | varchar |                                           |
| interview_date     | varchar |                                           |
| reporting_time     | varchar |                                           |
| required_documents | jsonb   |                                           |

#### `provider_results`

| Column         | Type                    | Notes                   |
| -------------- | ----------------------- | ----------------------- |
| id             | uint PK                 |                         |
| provider_id    | uint, INDEX, NOT NULL   |                         |
| scholarship_id | uint, INDEX             |                         |
| title          | varchar NOT NULL        |                         |
| status         | varchar DEFAULT 'draft' |                         |
| published_at   | timestamptz NULLABLE    |                         |
| results        | jsonb                   | Array of result entries |

---

### 2.4 Institution Portal Domain

All tables have `institution_id` FK to `institution_users.id`.

| Table                              | Key Columns                                                                                                   | Purpose                   |
| ---------------------------------- | ------------------------------------------------------------------------------------------------------------- | ------------------------- |
| `institution_programs`             | name, description, duration, fee, eligibility, capacity, banner_url, data(JSONB), status                      | Academic programs         |
| `institution_media`                | url, type, title                                                                                              | Gallery/media items       |
| `institution_counselling_sessions` | title, description, scheduled_at, duration, max_seats, booked_seats, status                                   | Counselling session slots |
| `institution_counselling_bookings` | session_id, user_id, status, student_name/phone/email, session_mode                                           | Booked sessions           |
| `institution_entrances`            | title, description, program, date, marks, seats, questions(JSONB), ~15 JSONB fields                           | Entrance exams            |
| `institution_entrance_applicants`  | entrance_id, user_id, status, score, rank                                                                     | Entrance applicants       |
| `institution_events`               | name, short_desc, description, image_url, event_type, category, start/end_date, location, tags(JSONB), status | Events                    |
| `institution_news`                 | title, short_desc, content, image_url, news_type, tags(JSONB), status                                         | News articles             |
| `institution_blogs`                | title, content, excerpt, image, category, published                                                           | Blog posts                |
| `institution_qms`                  | title, description, category, status, score, documents(JSONB)                                                 | Quality management        |
| `institution_messages`             | subject, content, read, direction (to/from)                                                                   | Messaging with students   |
| `institution_settings`             | institution_id (UNIQUE), email_notifs, timezone, language, public_profile                                     | Settings                  |
| `admission_pages`                  | institution_id, title, status, published_at, data(JSONB)                                                      | Custom admission pages    |

#### `institution_subscriptions`

Linked to `institution_users` via `institution_id`.

| Column              | Type                      |
| ------------------- | ------------------------- |
| id                  | uint PK                   |
| institution_id      | uint, INDEX, NOT NULL     |
| status              | varchar DEFAULT 'pending' |
| start_date          | timestamptz NULLABLE      |
| expire_date         | timestamptz NULLABLE      |
| last_payment_date   | timestamptz NULLABLE      |
| last_payment_amount | float DEFAULT 0           |
| remarks             | varchar DEFAULT ''        |

---

### 2.5 Provider Content Domain (Scholarship Provider)

All tables have `provider_id` FK to `scholarship_provider_users.id`.

| Table                      | Key Columns                                                                                                                                                      |
| -------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `provider_news`            | title, short_desc, content, image_url(optional), news_type, published_by, tags(JSONB), status                                                                    |
| `provider_events`          | name, short_desc, description, image_url(optional), event_type, category, max_participants, start/end_date, location, tags(JSONB), status                        |
| `provider_blogs`           | title, content, image_url(optional), author, status, views, likes                                                                                                |
| `provider_services`        | icon, title, description, external_link, sort_order                                                                                                              |
| `provider_sectors`         | name, description, color, image_url, icon, external_link, sort_order                                                                                             |
| `provider_projects`        | title, description, image_url, category, external_link, date, sort_order                                                                                         |
| `provider_gallery_images`  | folder, image_url, caption, sort_order                                                                                                                           |
| `provider_reviews`         | author_name, avatar_url, rating(1-5), title, content, pros, cons, status                                                                                         |
| `provider_volunteers`      | slug, title, banner_image, description, volunteer_type, payment, date_mode, range_start/end, specific_dates(JSONB), deadline, districts(JSONB), active, location |
| `volunteer_applications`   | volunteer_id, user_id(optional), full_name, gender, phone, email, designation, address, available_days(JSONB), volunteered_before, cv_path, status               |
| `provider_calendar_events` | title, description, start/end_date, color, is_all_day                                                                                                            |
| `provider_notifications`   | provider_id, title, message, type, read, link                                                                                                                    |
| `provider_settings`        | provider_id(UNIQUE), email_notifs, sms_notifs, auto_reject, timezone, language                                                                                   |
| `provider_access`          | provider_id, email, role, status                                                                                                                                 |
| `provider_access_users`    | provider_id, name, email(UNIQUE), password, role, role_label, status, last_active, permissions(JSONB)                                                            |

---

### 2.6 Forum Domain

| Table                     | Key Columns                                                                                                                     | FK                                                        |
| ------------------------- | ------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------- |
| `forum_communities`       | name(UNIQUE), emoji, bg_color                                                                                                   | —                                                         |
| `forum_community_members` | community_id, user_id                                                                                                           | UNIQUE(community_id, user_id)                             |
| `forum_posts`             | user_id, community_id, category, title, content, image_url, video_url, poll_options, upvotes, downvotes, comment_count, is_poll | `user_id` → `users`, `community_id` → `forum_communities` |
| `forum_comments`          | post_id, user_id, content, parent_id(self-ref)                                                                                  | `post_id` → `forum_posts`                                 |
| `forum_votes`             | post_id, user_id, vote(smallint)                                                                                                | UNIQUE(post_id, user_id)                                  |
| `forum_saves`             | post_id, user_id                                                                                                                | UNIQUE(post_id, user_id)                                  |
| `forum_poll_votes`        | post_id, user_id, option_idx                                                                                                    | UNIQUE(post_id, user_id)                                  |

---

### 2.7 Student Dashboard Domain

| Table             | Key Columns                                                                        | FK                  |
| ----------------- | ---------------------------------------------------------------------------------- | ------------------- |
| `messages`        | sender_id, receiver_id, subject, content, read, direction                          | Both to `users.id`  |
| `calendar_events` | user_id, title, description, start/end_date, location, link, color, reminder, type | `user_id` → `users` |
| `sphere_invites`  | user_id, institution_id, title, message, status, type                              | `user_id` → `users` |
| `bookmarks`       | user_id, item_id, item_type                                                        | `user_id` → `users` |
| `notifications`   | user_id, title, message, type, read, link                                          | `user_id` → `users` |

---

### 2.8 Review Domain

| Table             | Key Columns                                                            | FK                                             |
| ----------------- | ---------------------------------------------------------------------- | ---------------------------------------------- |
| `reviews`         | user_id, college_id, rating(JSONB), title, content, pros, cons, status | `user_id` → `users`, `college_id` → `colleges` |
| `review_helpfuls` | user_id, review_id                                                     | UNIQUE(user_id, review_id)                     |
| `review_reports`  | user_id, review_id, reason                                             | UNIQUE(user_id, review_id)                     |

---

### 2.9 System Domain

| Table                  | Key Columns                                                                                                 |
| ---------------------- | ----------------------------------------------------------------------------------------------------------- |
| `contact_inquiries`    | name, email, phone, subject, message, type, status                                                          |
| `ads`                  | title, image_url, link_url, location, page, position, start/end_date, active, clicks, impressions, priority |
| `carousel_slides`      | page, title, subtitle, description, image_url, link_url, button_text, order, active                         |
| `public_notifications` | title, message, type, link, active, icon, color, bg_color                                                   |

---

### 2.10 Other Tables

| Table                  | Module         | Key Columns                                                                                                           |
| ---------------------- | -------------- | --------------------------------------------------------------------------------------------------------------------- |
| `site_pages`           | chat           | slug(UNIQUE), title, content, embedding(v1536)                                                                        |
| `feedback`             | feedback       | user_id, rating, experience, designation, email                                                                       |
| `counselling_bookings` | counselling    | user_id, college, program_level, interested_course, session_mode, session_date/time, student_name/phone/email, status |
| `admissions`           | admission      | user_id(optional), college_id, program_name/level, student_name/email/phone, status, documents(JSONB), notes          |
| `shiksha_applications` | projectshiksha | ~50+ personal info fields (standalone, no user FK)                                                                    |
| `shiksha_payments`     | projectshiksha | application_id, method, amount, status, transaction_id                                                                |

---

## 3. Key Relationship Summary

| Relationship                              | Type     | FK Column                                 | References                      |
| ----------------------------------------- | -------- | ----------------------------------------- | ------------------------------- |
| User → EducationEntry                     | 1:N      | `education_entries.user_id`               | `users.id`                      |
| University → College                      | 1:N      | `colleges.university_id`                  | `universities.id`               |
| InstitutionUser → College                 | N:1      | `institution_users.college_id`            | `colleges.id`                   |
| InstitutionUser → InstitutionSubscription | 1:1      | `subscriptions.institution_id`            | `institution_users.id`          |
| InstitutionUser → InstitutionPrograms     | 1:N      | `institution_programs.institution_id`     | `institution_users.id`          |
| ProviderUser → ProviderScholarship        | 1:N      | `provider_scholarships.provider_id`       | `scholarship_provider_users.id` |
| ProviderScholarship → ProviderApplication | 1:N      | `provider_applications.scholarship_id`    | `provider_scholarships.id`      |
| Scholarship → ScholarshipApplication      | 1:N      | `scholarship_applications.scholarship_id` | `scholarships.id`               |
| ForumPost → ForumComment                  | 1:N      | `forum_comments.post_id`                  | `forum_posts.id`                |
| ForumComment → ForumComment               | self-ref | `forum_comments.parent_id`                | `forum_comments.id`             |
| ForumPost → ForumVote                     | 1:N      | `forum_votes.post_id`                     | `forum_posts.id`                |
| ForumCommunity → ForumCommunityMember     | 1:N      | `forum_community_members.community_id`    | `forum_communities.id`          |
| Review → ReviewHelpful                    | 1:N      | `review_helpfuls.review_id`               | `reviews.id`                    |
| College → Review                          | 1:N      | `reviews.college_id`                      | `colleges.id`                   |
| User → Review                             | 1:N      | `reviews.user_id`                         | `users.id`                      |
| Admission → College                       | N:1      | `admissions.college_id`                   | `colleges.id`                   |
| Admission → User                          | N:1      | `admissions.user_id`                      | `users.id`                      |

---

## 4. JSONB Column Usage

~40+ JSONB columns across the schema. These store denormalized/complex data:

| Table                      | JSONB Columns                | Content                                                                                                                                                     |
| -------------------------- | ---------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `users`                    | preferences                  | Onboarding flow, education level, interests                                                                                                                 |
| `institution_users`        | profile_access, profile_data | Section visibility, complex profile sections                                                                                                                |
| `colleges`                 | 15+ columns                  | featured_programs, amenities, courses, scholarships, gallery, about, admissions, alumni, departments, reviews, etc.                                         |
| `universities`             | 15+ columns                  | about, contact, overview, leadership, courses, programs, events, etc.                                                                                       |
| `scholarships`             | 25+ columns                  | field_of_study, selection_process, eligibility_criteria, required_documents, timeline, benefits, FAQs, form_config, payment_config, gallery, partners, etc. |
| `provider_scholarships`    | 20+ columns                  | Same structure as `scholarships`                                                                                                                            |
| `scholarship_applications` | documents                    | Uploaded document metadata                                                                                                                                  |
| `provider_applications`    | documents                    | Uploaded document metadata                                                                                                                                  |
| `institution_entrances`    | 10+ columns                  | overview_details, exam_date_schedules, eligibility_list, application_steps, exam_pattern, subject_marks, model_sets, etc.                                   |
| `provider_volunteers`      | specific_dates, districts    | Volunteer schedule and coverage                                                                                                                             |
| `volunteer_applications`   | available_days               | Days available to volunteer                                                                                                                                 |
| `provider_access_users`    | permissions                  | Granular feature permissions                                                                                                                                |
| `provider_results`         | results                      | Array of result entries                                                                                                                                     |
| `written_exam_results`     | required_documents           | Documents needed for interview                                                                                                                              |
| `admission_pages`          | data                         | Page content as JSON                                                                                                                                        |
| `institution_events`       | tags                         | Event tags                                                                                                                                                  |
| `institution_news`         | tags                         | News tags                                                                                                                                                   |
| `institution_programs`     | data                         | Program extended data                                                                                                                                       |
| `institution_qms`          | documents                    | QMS document references                                                                                                                                     |

---

## 5. Tables with `embedding vector(1536)` Columns

These support hybrid vector + keyword search (pgvector):

| Table          | Embedding Source Text                                             |
| -------------- | ----------------------------------------------------------------- |
| `colleges`     | name, full_name, description, location, affiliation, college_type |
| `courses`      | title, short_title, description, field, level, affiliation        |
| `exams`        | title, description, board, type, university                       |
| `scholarships` | title, description, provider, location, scholarship_type          |
| `news`         | title, excerpt, content, category, source                         |
| `events`       | title, description, excerpt, category, location                   |
| `blogs`        | title, excerpt, content, category, author                         |
| `site_pages`   | title, content                                                    |

---

## 6. Notable Schema Characteristics

**Three separate user tables** instead of a single `users` table with a type discriminator. This means:

- No shared sequence for user IDs across roles
- Education entries only for `users` (students), not institutions/providers
- Each login flow checks the correct table
- Email uniqueness is enforced per-table but cross-table checks are done in service code

**Dual scholarship tables** (`scholarships` + `provider_scholarships`) with near-identical schemas. Platform-admin scholarships live in `scholarships`, provider-owned ones in `provider_scholarships`. A `provider_scholarship_id` column on `scholarships` links them when synced.

**In-memory only models** — `Payment` and `ProviderPayment` are defined in Go structs with `gorm:"-"` tags and are NOT persisted to the database. Payment data flows through eSewa/Khalti API and is tracked via application status fields.

**JSONB for flexibility** — Many complex data structures (college profiles, university pages, scholarship details) are stored as JSONB rather than normalized tables, giving the frontend flexibility without schema migrations.

**No cross-table referential actions** — GORM's `AutoMigrate` does not create ON DELETE CASCADE. Soft deletes (`deleted_at`) are used throughout.
