# PSScriptAnalyzer: all default rules except ExcludeRules.
# Each excluded rule has a comment that states why it is off.

@{
    Severity     = @('Error', 'Warning', 'Information')
    ExcludeRules = @(
        'PSDSCDscExamplesPresent'                      # Not a DSC resource module.
        'PSDSCDscTestsPresent'                         # Not a DSC resource module.
        'PSDSCReturnCorrectTypesForDSCFunctions'       # Not a DSC resource module.
        'PSDSCUseVerboseMessageInDSCResource'          # Not a DSC resource module.
        'PSDSCStandardDSCFunctionsInResource'          # Not a DSC resource module.
        'PSDSCUseIdenticalMandatoryParametersForDSC'   # Not a DSC resource module.
        'PSDSCUseIdenticalParametersForDSC'            # Not a DSC resource module.
        'PSMissingModuleManifestField'                 # Not a PowerShell module; no .psd1 manifest.
        'PSUseToExportFieldsInManifest'                # Not a PowerShell module; no export list.
        'PSAvoidUsingDeprecatedManifestFields'         # Not a PowerShell module; no manifest fields.
        'PSUseUTF8EncodingForHelpFile'                 # No module help files in this repository.
        'PSProvideCommentHelp'                         # Not exported cmdlets; comment-based help is not the interface.
        'PSUseShouldProcessForStateChangingFunctions'  # Not shipped cmdlets; no -WhatIf contract.
        'PSShouldProcess'                              # Not shipped cmdlets; no ShouldProcess body.
        'PSUseSupportsShouldProcess'                   # Not shipped cmdlets; no SupportsShouldProcess.
        'PSAvoidShouldContinueWithoutForce'            # No ShouldContinue prompts; GUI/console uses MessageBox and Read-Host.
        'PSAvoidUsingWriteHost'                        # Console manager, GUI prompts, and tests print to the host on purpose.
        'PSUseSingularNouns'                           # Zapret, Winws, and Ipset are product names, not nouns to inflect.
        'PSUseBOMForUnicodeEncodedFile'                # UTF-8 without BOM is the encoding for these scripts.
        'PSAvoidUsingWMICmdlet'                        # check-env.ps1 uses Get-WmiObject only after CIM fails (Windows 7).
        'PSUseConstrainedLanguageMode'                 # Target is full-language desktop Windows PowerShell, not CLM.
        'PSUseCompatibleCommands'                      # Needs a 5.1 command profile; syntax is pinned to 5.1 below.
        'PSUseCompatibleCmdlets'                       # Needs a 5.1 cmdlet profile; syntax is pinned to 5.1 below.
        'PSUseCompatibleTypes'                         # Needs a 5.1 type profile; syntax is pinned to 5.1 below.
        'PSUseOutputTypeCorrectly'                     # [OutputType()] is for pipeline cmdlets, not these scripts.
        'PSUseProcessBlockForPipelineCommand'          # process {} is for pipeline cmdlets, not these scripts.
        'PSAlignAssignmentStatement'                   # Formatter/style, not a correctness check.
        'PSPlaceCloseBrace'                            # Formatter/style, not a correctness check.
        'PSPlaceOpenBrace'                             # Formatter/style, not a correctness check.
        'PSUseConsistentIndentation'                   # Formatter/style, not a correctness check.
        'PSUseConsistentWhitespace'                    # Formatter/style, not a correctness check.
        'PSUseConsistentParametersKind'                # Formatter/style, not a correctness check.
        'PSAvoidLongLines'                             # Formatter/style; strategy here-strings are long on purpose.
        'PSAvoidSemicolonsAsLineTerminators'           # Formatter/style, not a correctness check.
        'PSAvoidExclaimOperator'                       # Formatter/style; -not is already the usual form.
        'PSAvoidUsingDoubleQuotesForConstantString'    # Formatter/style, not a correctness check.
        'PSUseCorrectCasing'                           # Formatter/style, not a correctness check.
    )
    Rules        = @{
        PSUseCompatibleSyntax = @{
            Enable         = $true
            TargetVersions = @('5.1')
        }
    }
}
