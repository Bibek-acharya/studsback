// Quick one-shot test for admit card PDF generation.
// Run from the studsback directory:
//   go run scratch/test_admitcard.go
package main

import (
	"fmt"
	"os"
	"studsphere/backend/internal/scholarship"
)

func main() {
	fmt.Println("Generating admit card PDF with dummy data...")

	data := scholarship.AdmitCardData{
		CandidateName:    "Siddhartha Gautam",
		DateOfBirth:      "14-Sep-2005",
		Gender:           "Male",
		RollNumber:       "PS-830456129",
		ExamCentre:       "Kathmandu Model College, Bagbazar",
		Stream:           "Science",
		PhotoURL:         "https://encrypted-tbn0.gstatic.com/images?q=tbn:ANd9GcRpT_ljsPtUre6DcAfZvx-obUMoXfNR8U3x-Q&s",
		ScholarshipTitle: "Project Shiksha 2083",
		Provider:         "Hundred Group Nepal",
		ExamDate:         "25.01.2083",
		ExamTime:         "09:00 A.M. To 11:30 A.M.",
		Shift:            "1st",
		SubjectName:      "Science",
	}

	pdfBytes, err := scholarship.GenerateAdmitCardPDF(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	outFile := "test_admit_card.pdf"
	if err := os.WriteFile(outFile, pdfBytes, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ PDF generated successfully: %s (%d bytes)\n", outFile, len(pdfBytes))
	fmt.Println("Open it with: xdg-open test_admit_card.pdf")
}
