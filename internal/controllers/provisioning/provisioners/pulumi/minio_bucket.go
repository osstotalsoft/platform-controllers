package pulumi

import (
	"fmt"

	"github.com/pulumi/pulumi-minio/sdk/go/minio"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"totalsoft.ro/platform-controllers/internal/controllers/provisioning"
	platformv1 "totalsoft.ro/platform-controllers/pkg/apis/platform/v1alpha1"
	provisioningv1 "totalsoft.ro/platform-controllers/pkg/apis/provisioning/v1alpha1"
)

func deployMinioBucket(target provisioning.ProvisioningTarget,
	minioBucket *provisioningv1.MinioBucket,
	dependencies []pulumi.Resource,
	ctx *pulumi.Context) (*minio.S3Bucket, error) {

	valueExporter := handleValueExport(target)
	gvk := provisioningv1.SchemeGroupVersion.WithKind("MinioBucket")

	bucketName := provisioning.MatchTarget(target,
		func(tenant *platformv1.Tenant) string {
			return fmt.Sprintf("%s-%s-%s", minioBucket.Spec.BucketName, tenant.Spec.PlatformRef, tenant.GetName())
		},
		func(platform *platformv1.Platform) string {
			return fmt.Sprintf("%s-%s", minioBucket.Spec.BucketName, platform.GetName())
		},
	)

	userName := fmt.Sprintf("%s-%s-%s", provisioning.MatchTarget(target,
		func(tenant *platformv1.Tenant) string {
			return fmt.Sprintf("%s-%s", tenant.Spec.PlatformRef, tenant.GetName())
		},
		func(platform *platformv1.Platform) string {
			return fmt.Sprintf("%s", platform.GetName())
		},
	), minioBucket.Spec.DomainRef, minioBucket.Spec.BucketName)

	user, err := minio.NewIamUser(ctx, userName, &minio.IamUserArgs{
		ForceDestroy: pulumi.BoolPtr(true),
		Name:         pulumi.String(userName),
	}, pulumi.DependsOn(dependencies))
	if err != nil {
		return nil, err
	}

	minio.NewIamUserPolicyAttachment(ctx, userName, &minio.IamUserPolicyAttachmentArgs{
		UserName:   user.Name,
		PolicyName: pulumi.String("readwrite"),
	}, pulumi.DependsOn(dependencies), pulumi.Parent(user))

	sa, err := minio.NewIamServiceAccount(ctx, userName, &minio.IamServiceAccountArgs{
		Policy: pulumi.String(fmt.Sprintf(`{
 "Version": "2012-10-17",
 "Statement": [
  {
   "Effect": "Allow",
   "Action": [
    "s3:*"
   ],
   "Resource": [
    "arn:aws:s3:::%s",
    "arn:aws:s3:::%s/*"
   ]
  }
 ]
}`, bucketName, bucketName)),
		TargetUser: user.Name,
	}, pulumi.DependsOn(dependencies))
	if err != nil {
		return nil, err
	}

	pulumiRetainOnDelete := provisioning.GetDeletePolicy(target) == platformv1.DeletePolicyRetainStatefulResources
	ignoreChanges := []string{}
	if pulumiRetainOnDelete {
		ignoreChanges = []string{"bucket"}
	}

	bucket, err := minio.NewS3Bucket(ctx, minioBucket.Name, &minio.S3BucketArgs{
		Acl:          nil,
		Bucket:       pulumi.String(bucketName),
		ForceDestroy: pulumi.Bool(!pulumiRetainOnDelete),
	},
		pulumi.RetainOnDelete(pulumiRetainOnDelete),
		pulumi.IgnoreChanges(ignoreChanges),
		pulumi.DependsOn(dependencies))
	if err != nil {
		return nil, err
	}

	for _, exp := range minioBucket.Spec.Exports {
		domain := exp.Domain
		if domain == "" {
			domain = minioBucket.Spec.DomainRef
		}

		err = valueExporter(newExportContext(ctx, domain, minioBucket.Name, minioBucket.ObjectMeta, gvk),
			map[string]exportTemplateWithValue{
				"accessKey":  {exp.AccessKey, sa.AccessKey},
				"secretKey":  {exp.SecretKey, sa.SecretKey},
				"bucketName": {exp.BucketName, bucket.Bucket},
			})
		if err != nil {
			return nil, err
		}
	}
	return bucket, nil
}
