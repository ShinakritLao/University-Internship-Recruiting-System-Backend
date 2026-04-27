package main

import (
    "regexp"
    "unicode"
)

func isValidEmail(email string) bool {
    re := regexp.MustCompile(`^[a-zA-Z0-9._%%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
    return re.MatchString(email)
}

//-------------------------------------------------------------------------------------------------------//

func isValidPassword(p string) bool {
    if len(p) < 8 {
        return false
    }

    count := 0
    for _, c := range p {
        if unicode.IsDigit(c) {
            count++
        }
    }
    return count >= 2
}

//-------------------------------------------------------------------------------------------------------//

func detectRole(id string) string {
    if len(id) == 11 {
        return "student"
    }

    if len(id) == 8 {
        return "company"
    }

    if len(id) == 9 {
        return "staff"
    }

    return "invalid"
}