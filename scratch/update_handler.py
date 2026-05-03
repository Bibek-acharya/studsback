import sys

file_path = '/home/durgesh/Work/studsphere/studsback/internal/scholarship/handler.go'

with open(file_path, 'r') as f:
    content = f.read()

old_block = """func toScholarshipDetailResponse(s Scholarship) gin.H {
	return gin.H{
		"id":                   s.ID,
		"title":                s.Title,
		"provider":             s.Provider,
		"location":             s.Location,
		"value":                s.Value,
		"deadline":             formatDeadline(s.Deadline),
		"degree_level":         s.DegreeLevel,
		"funding_type":         s.FundingType,
		"scholarship_type":     s.ScholarshipType,
		"description":          s.Description,
		"image_url":            s.ImageURL,
		"status":               deriveScholarshipStatus(s.Deadline),
		"field_of_study":       parseStringArray(s.FieldOfStudy),
		"selection_process":    parseDetailFieldArray(s.SelectionProcess),
		"eligibility_criteria": parseDetailFieldArray(s.EligibilityCriteria),
		"excluded_regions":     parseStringArray(s.ExcludedRegions),
		"required_documents":   parseDetailFieldArray(s.RequiredDocuments),
		"timeline":             parseDetailFieldArray(s.Timeline),
		"benefits":             parseDetailFieldArray(s.Benefits),
		"faqs":                 parseDetailFieldArray(s.FAQs),
	}
}"""

new_block = """func toScholarshipDetailResponse(s Scholarship) gin.H {
	return gin.H{
		"id":                   s.ID,
		"title":                s.Title,
		"provider":             s.Provider,
		"location":             s.Location,
		"value":                s.Value,
		"deadline":             formatDeadline(s.Deadline),
		"degree_level":         s.DegreeLevel,
		"funding_type":         s.FundingType,
		"scholarship_type":     s.ScholarshipType,
		"description":          s.Description,
		"image_url":            s.ImageURL,
		"status":               deriveScholarshipStatus(s.Deadline),
		"field_of_study":       parseStringArray(s.FieldOfStudy),
		"selection_process":    parseDetailFieldArray(s.SelectionProcess),
		"eligibility_criteria": parseDetailFieldArray(s.EligibilityCriteria),
		"excluded_regions":     parseStringArray(s.ExcludedRegions),
		"required_documents":   parseStringArray(s.RequiredDocuments),
		"timeline":             parseDetailFieldArray(s.Timeline),
		"benefits":             parseDetailFieldArray(s.Benefits),
		"faqs":                 parseDetailFieldArray(s.FAQs),
		"provider_name":        s.ProviderName,
		"funding_type_other":   s.FundingTypeOther,
		"scholarship_type_other": s.ScholarshipTypeOther,
		"education_level":       s.EducationLevel,
		"education_level_other": s.EducationLevelOther,
		"apply_link":           s.ApplyLink,
		"coverage_area":        s.CoverageArea,
		"contact_email":        s.ContactEmail,
		"primary_phone":        s.PrimaryPhone,
		"secondary_phone":      s.SecondaryPhone,
		"website_url":          s.WebsiteUrl,
		"office_address":       s.OfficeAddress,
		"map_url":              s.MapUrl,
		"about_paragraph_1":    s.AboutParagraph1,
		"video_tutorials":      parseDetailFieldArray(s.VideoTutorials),
		"journey_timeline":     parseDetailFieldArray(s.JourneyTimeline),
		"scholarship_section_title": s.ScholarshipSectionTitle,
		"scholarship_subtitle":      s.ScholarshipSubtitle,
		"scholarship_description_1": s.ScholarshipDescription1,
		"scholarship_description_2": s.ScholarshipDescription2,
		"scholarship_types":         parseDetailFieldArray(s.ScholarshipTypes),
		"scholarship_types_new":     parseDetailFieldArray(s.ScholarshipTypesNew),
		"selection_rubric":          parseDetailFieldArray(s.SelectionRubric),
		"selection_rubric_new":       parseDetailFieldArray(s.SelectionRubricNew),
		"eligibility_section_title": s.EligibilitySectionTitle,
		"eligibility_subtitle":      s.EligibilitySubtitle,
		"basic_eligibility_criteria": parseStringArray(s.BasicEligibilityCriteria),
		"fully_funded_criteria":      parseStringArray(s.FullyFundedCriteria),
		"partially_funded_criteria":  parseStringArray(s.PartiallyFundedCriteria),
		"selection_process_steps":    parseDetailFieldArray(s.SelectionProcessSteps),
		"faqs_new":                  parseDetailFieldArray(s.FAQsNew),
		"gallery_images":            parseDetailFieldArray(s.GalleryImages),
		"gallery_images_new":         parseDetailFieldArray(s.GalleryImagesNew),
		"partner_groups":            parseDetailFieldArray(s.PartnerGroups),
		"exam_centers":              parseDetailFieldArray(s.ExamCenters),
		"exam_centers_new":           parseDetailFieldArray(s.ExamCentersNew),
		"downloads":                parseDetailFieldArray(s.Downloads),
	}
}"""

if old_block in content:
    content = content.replace(old_block, new_block)
    print("Replaced toScholarshipDetailResponse block")
elif old_block.replace('    ', '\\t').replace('\\t', '\t') in content:
    content = content.replace(old_block.replace('    ', '\\t').replace('\\t', '\t'), new_block.replace('    ', '\\t').replace('\\t', '\t'))
    print("Replaced toScholarshipDetailResponse block (tabs)")
else:
    print("Could not find toScholarshipDetailResponse block")
    sys.exit(1)

with open(file_path, 'w') as f:
    f.write(content)
