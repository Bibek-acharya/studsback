import sys

file_path = '/home/durgesh/Work/studsphere/studsback/internal/scholarshipprovider/service.go'

with open(file_path, 'r') as f:
    content = f.read()

old_block = """	publicScholarship := &publicscholarship.Scholarship{
		Title:                    scholarship.Title,
		Provider:                 provider.ProviderName,
		Location:                 scholarship.Location,
		Value:                    scholarship.Value,
		Deadline:                 scholarship.Deadline,
		DegreeLevel:              scholarship.DegreeLevel,
		FundingType:              scholarship.FundingType,
		ScholarshipType:          scholarship.ScholarshipType,
		Description:              scholarship.Description,
		BannerBackgroundImageURL: scholarship.BannerBackgroundImageURL,
		FieldOfStudy:             scholarship.FieldOfStudy,
		EligibilityCriteria:      scholarship.BasicEligibilityCriteria,
		RequiredDocuments:        scholarship.RequiredDocuments,
		PaymentConfig:            scholarship.PaymentConfig,
		ProviderScholarshipID:    &scholarship.ID,
	}"""

new_block = """	publicScholarship := &publicscholarship.Scholarship{
		Title:                    scholarship.Title,
		Provider:                 provider.ProviderName,
		Location:                 scholarship.Location,
		Value:                    scholarship.Value,
		Deadline:                 scholarship.Deadline,
		DegreeLevel:              scholarship.DegreeLevel,
		FundingType:              scholarship.FundingType,
		ScholarshipType:          scholarship.ScholarshipType,
		Description:              scholarship.Description,
		ImageURL:                 scholarship.BannerBackgroundImageURL,
		BannerBackgroundImageURL: scholarship.BannerBackgroundImageURL,
		FieldOfStudy:             scholarship.FieldOfStudy,
		EligibilityCriteria:      scholarship.BasicEligibilityCriteria,
		RequiredDocuments:        scholarship.RequiredDocuments,
		PaymentConfig:            scholarship.PaymentConfig,
		ProviderScholarshipID:    &scholarship.ID,
		ProviderName:             scholarship.ProviderName,
		FundingTypeOther:         scholarship.FundingTypeOther,
		ScholarshipTypeOther:     scholarship.ScholarshipTypeOther,
		EducationLevel:           scholarship.EducationLevel,
		EducationLevelOther:      scholarship.EducationLevelOther,
		ApplyLink:                scholarship.ApplyLink,
		CoverageArea:             scholarship.CoverageArea,
		ContactEmail:             scholarship.ContactEmail,
		PrimaryPhone:             scholarship.PrimaryPhone,
		SecondaryPhone:           scholarship.SecondaryPhone,
		WebsiteUrl:               scholarship.WebsiteUrl,
		OfficeAddress:            scholarship.OfficeAddress,
		MapUrl:                   scholarship.MapUrl,
		AboutParagraph1:          scholarship.AboutParagraph1,
		VideoTutorials:           scholarship.VideoTutorials,
		JourneyTimeline:          scholarship.JourneyTimeline,
		Timeline:                 scholarship.Timeline,
		ScholarshipSectionTitle:  scholarship.ScholarshipSectionTitle,
		ScholarshipSubtitle:      scholarship.ScholarshipSubtitle,
		ScholarshipDescription1:  scholarship.ScholarshipDescription1,
		ScholarshipDescription2:  scholarship.ScholarshipDescription2,
		ScholarshipTypes:         scholarship.ScholarshipTypes,
		ScholarshipTypesNew:      scholarship.ScholarshipTypesNew,
		SelectionRubric:          scholarship.SelectionRubric,
		SelectionRubricNew:       scholarship.SelectionRubricNew,
		EligibilitySectionTitle:  scholarship.EligibilitySectionTitle,
		EligibilitySubtitle:      scholarship.EligibilitySubtitle,
		BasicEligibilityCriteria: scholarship.BasicEligibilityCriteria,
		FullyFundedCriteria:      scholarship.FullyFundedCriteria,
		PartiallyFundedCriteria:  scholarship.PartiallyFundedCriteria,
		SelectionProcessSteps:    scholarship.SelectionProcessSteps,
		FAQsNew:                  scholarship.FAQsNew,
		GalleryImages:            scholarship.GalleryImages,
		GalleryImagesNew:         scholarship.GalleryImagesNew,
		PartnerGroups:            scholarship.PartnerGroups,
		ExamCenters:              scholarship.ExamCenters,
		ExamCentersNew:           scholarship.ExamCentersNew,
		Downloads:                scholarship.Downloads,
	}"""

if old_block in content:
    content = content.replace(old_block, new_block)
    print("Replaced publicScholarship block")
else:
    # Try with tabs
    old_block_tabs = old_block.replace('    ', '\\t').replace('\\t', '\t')
    if old_block_tabs in content:
        content = content.replace(old_block_tabs, new_block.replace('    ', '\\t').replace('\\t', '\t'))
        print("Replaced publicScholarship block (tabs)")
    else:
        print("Could not find publicScholarship block")
        sys.exit(1)

old_updates = """		updates := map[string]interface{}{
			"title":                       publicScholarship.Title,
			"provider":                    publicScholarship.Provider,
			"location":                    publicScholarship.Location,
			"value":                       publicScholarship.Value,
			"deadline":                    publicScholarship.Deadline,
			"application_start_date":      publicScholarship.ApplicationStartDate,
			"degree_level":                publicScholarship.DegreeLevel,
			"funding_type":                publicScholarship.FundingType,
			"scholarship_type":            publicScholarship.ScholarshipType,
			"description":                 publicScholarship.Description,
			"banner_background_image_url": publicScholarship.BannerBackgroundImageURL,
			"field_of_study":              publicScholarship.FieldOfStudy,
			"eligibility_criteria":        publicScholarship.EligibilityCriteria,
			"required_documents":          publicScholarship.RequiredDocuments,
			"payment_config":              publicScholarship.PaymentConfig,
			"provider_scholarship_id":     scholarship.ID,
		}"""

new_updates = """		updates := map[string]interface{}{
			"title":                       publicScholarship.Title,
			"provider":                    publicScholarship.Provider,
			"location":                    publicScholarship.Location,
			"value":                       publicScholarship.Value,
			"deadline":                    publicScholarship.Deadline,
			"degree_level":                publicScholarship.DegreeLevel,
			"funding_type":                publicScholarship.FundingType,
			"scholarship_type":            publicScholarship.ScholarshipType,
			"description":                 publicScholarship.Description,
			"image_url":                   publicScholarship.ImageURL,
			"banner_background_image_url": publicScholarship.BannerBackgroundImageURL,
			"field_of_study":              publicScholarship.FieldOfStudy,
			"eligibility_criteria":        publicScholarship.EligibilityCriteria,
			"required_documents":          publicScholarship.RequiredDocuments,
			"payment_config":              publicScholarship.PaymentConfig,
			"provider_scholarship_id":     scholarship.ID,
			"provider_name":               publicScholarship.ProviderName,
			"funding_type_other":         publicScholarship.FundingTypeOther,
			"scholarship_type_other":     publicScholarship.ScholarshipTypeOther,
			"education_level":           publicScholarship.EducationLevel,
			"education_level_other":      publicScholarship.EducationLevelOther,
			"apply_link":                publicScholarship.ApplyLink,
			"coverage_area":             publicScholarship.CoverageArea,
			"contact_email":             publicScholarship.ContactEmail,
			"primary_phone":             publicScholarship.PrimaryPhone,
			"secondary_phone":           publicScholarship.SecondaryPhone,
			"website_url":               publicScholarship.WebsiteUrl,
			"office_address":            publicScholarship.OfficeAddress,
			"map_url":                   publicScholarship.MapUrl,
			"about_paragraph_1":          publicScholarship.AboutParagraph1,
			"video_tutorials":           publicScholarship.VideoTutorials,
			"journey_timeline":          publicScholarship.JourneyTimeline,
			"timeline":                 publicScholarship.Timeline,
			"scholarship_section_title": publicScholarship.ScholarshipSectionTitle,
			"scholarship_subtitle":      publicScholarship.ScholarshipSubtitle,
			"scholarship_description_1": publicScholarship.ScholarshipDescription1,
			"scholarship_description_2": publicScholarship.ScholarshipDescription2,
			"scholarship_types":         publicScholarship.ScholarshipTypes,
			"scholarship_types_new":     publicScholarship.ScholarshipTypesNew,
			"selection_rubric":          publicScholarship.SelectionRubric,
			"selection_rubric_new":       publicScholarship.SelectionRubricNew,
			"eligibility_section_title": publicScholarship.EligibilitySectionTitle,
			"eligibility_subtitle":      publicScholarship.EligibilitySubtitle,
			"basic_eligibility_criteria": publicScholarship.BasicEligibilityCriteria,
			"fully_funded_criteria":      publicScholarship.FullyFundedCriteria,
			"partially_funded_criteria":  publicScholarship.PartiallyFundedCriteria,
			"selection_process_steps":    publicScholarship.SelectionProcessSteps,
			"faqs_new":                  publicScholarship.FAQsNew,
			"gallery_images":            publicScholarship.GalleryImages,
			"gallery_images_new":         publicScholarship.GalleryImagesNew,
			"partner_groups":            publicScholarship.PartnerGroups,
			"exam_centers":              publicScholarship.ExamCenters,
			"exam_centers_new":           publicScholarship.ExamCentersNew,
			"downloads":                publicScholarship.Downloads,
		}"""

if old_updates in content:
    content = content.replace(old_updates, new_updates)
    print("Replaced updates block")
else:
    old_updates_tabs = old_updates.replace('    ', '\\t').replace('\\t', '\t')
    if old_updates_tabs in content:
        content = content.replace(old_updates_tabs, new_updates.replace('    ', '\\t').replace('\\t', '\t'))
        print("Replaced updates block (tabs)")
    else:
        print("Could not find updates block")
        sys.exit(1)

with open(file_path, 'w') as f:
    f.write(content)
